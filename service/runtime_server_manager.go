//
// DISCLAIMER
//
// Copyright 2017-2021 ArangoDB GmbH, Cologne, Germany
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Copyright holder is ArangoDB GmbH, Cologne, Germany
//
// Author Ewout Prangsma
// Author Tomasz Mielech
//

package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/arangodb-helper/arangodb/pkg/definitions"
	"github.com/arangodb-helper/arangodb/pkg/logging"
	"github.com/arangodb-helper/arangodb/service/actions"
)

// runtimeServerManager implements the start, monitor, stop behavior of database servers in a runtime
// state.
type runtimeServerManager struct {
	logMutex        sync.Mutex // Mutex used to synchronize server log output
	agentProc       ProcessWrapper
	dbserverProc    ProcessWrapper
	coordinatorProc ProcessWrapper
	singleProc      ProcessWrapper
	syncMasterProc  ProcessWrapper
	syncWorkerProc  ProcessWrapper

	stopping bool
}

// runtimeServerManagerContext provides a context for the runtimeServerManager.
type runtimeServerManagerContext interface {
	ClientBuilder

	// ClusterConfig returns the current cluster configuration and the current peer
	ClusterConfig() (ClusterConfig, *Peer, ServiceMode)

	// serverPort returns the port number on which my server of given type will listen.
	serverPort(serverType definitions.ServerType) (int, error)

	// serverHostDir returns the path of the folder (in host namespace) containing data for the given server.
	serverHostDir(serverType definitions.ServerType) (string, error)
	// serverContainerDir returns the path of the folder (in container namespace) containing data for the given server.
	serverContainerDir(serverType definitions.ServerType) (string, error)

	// serverHostLogFile returns the path of the logfile (in host namespace) to which the given server will write its logs.
	serverHostLogFile(serverType definitions.ServerType) (string, error)
	// serverContainerLogFile returns the path of the logfile (in container namespace) to which the given server will write its logs.
	serverContainerLogFile(serverType definitions.ServerType) (string, error)

	// removeRecoveryFile removes any recorded RECOVERY file.
	removeRecoveryFile()

	// UpgradeManager returns the upgrade manager service.
	UpgradeManager() UpgradeManager

	// TestInstance checks the `up` status of an arangod server instance.
	TestInstance(ctx context.Context, serverType definitions.ServerType, address string, port int,
		statusChanged chan StatusItem) (up, correctRole bool, version, role, mode string, isLeader bool, statusTrail []int, cancelled bool)

	// IsLocalSlave returns true if this peer is running as a local slave
	IsLocalSlave() bool

	// DatabaseFeatures returns the detected database features.
	DatabaseFeatures() DatabaseFeatures

	// Stop the peer
	Stop()
}

// startServer starts a single Arangod/Arangosync server of the given type.
func startServer(ctx context.Context, log zerolog.Logger, runtimeContext runtimeServerManagerContext, runner Runner,
	config Config, bsCfg BootstrapConfig, myHostAddress string, serverType definitions.ServerType, features DatabaseFeatures, restart int) (Process, bool, error) {
	myPort, err := runtimeContext.serverPort(serverType)
	if err != nil {
		return nil, false, maskAny(err)
	}
	myHostDir, err := runtimeContext.serverHostDir(serverType)
	if err != nil {
		return nil, false, maskAny(err)
	}
	myContainerDir, err := runtimeContext.serverContainerDir(serverType)
	if err != nil {
		return nil, false, maskAny(err)
	}
	myContainerLogFile, err := runtimeContext.serverContainerLogFile(serverType)
	if err != nil {
		return nil, false, maskAny(err)
	}

	os.MkdirAll(filepath.Join(myHostDir, "data"), 0755)
	os.MkdirAll(filepath.Join(myHostDir, "apps"), 0755)
	os.MkdirAll(filepath.Join(myHostDir, definitions.ArangodJWTSecretFolderName), 0700)

	// Check if the server is already running
	log.Info().Msgf("Looking for a running instance of %s on port %d", serverType, myPort)
	p, err := runner.GetRunningServer(myHostDir)
	if err != nil {
		return nil, false, maskAny(err)
	}
	if p != nil {
		log.Info().Msgf("%s seems to be running already, checking port %d...", serverType, myPort)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		up, correctRole, _, _, _, _, _, _ := runtimeContext.TestInstance(ctx, serverType, myHostAddress, myPort, nil)
		cancel()
		if up && correctRole {
			log.Info().Msgf("%s is already running on %d. No need to start anything.", serverType, myPort)
			return p, false, nil
		} else if !up {
			log.Info().Msgf("%s is not up on port %d. Terminating existing process and restarting it...", serverType, myPort)
		} else if !correctRole {
			expectedRole, expectedMode := serverType.ExpectedServerRole()
			log.Info().Msgf("%s is not of role '%s.%s' on port %d. Terminating existing process and restarting it...", serverType, expectedRole, expectedMode, myPort)
		}

		// Terminate without actions
		terminateProcessWithActions(log, p, serverType, 0, time.Minute)
	}

	// Check availability of port
	if !WaitUntilPortAvailable("", myPort, time.Second*3) {
		return nil, true, maskAny(fmt.Errorf("Cannot start %s, because port %d is already in use", serverType, myPort))
	}

	log.Info().Msgf("Starting %s on port %d", serverType, myPort)
	processType := serverType.ProcessType()
	// Create/read arangod.conf
	var confVolumes []Volume
	var arangodConfig configFile
	var containerSecretFileName string
	if processType == definitions.ProcessTypeArangod {
		var err error
		confVolumes, arangodConfig, err = createArangodConf(log, bsCfg, myHostDir, myContainerDir, strconv.Itoa(myPort), serverType, features)
		if err != nil {
			return nil, false, maskAny(err)
		}
	}
	if processType == definitions.ProcessTypeArangoSync || features.HasJWTSecretFileOption() {
		var err error
		var secretFileVolumes []Volume
		secretFileVolumes, containerSecretFileName, err = createArangoClusterSecretFile(log, bsCfg, myHostDir, myContainerDir, serverType, features)
		if err != nil {
			return nil, false, maskAny(err)
		}
		confVolumes = append(confVolumes, secretFileVolumes...)
	}

	// Collect volumes
	v := collectServerConfigVolumes(serverType, arangodConfig)
	confVolumes = append(confVolumes, v...)

	// Create server command line arguments
	clusterConfig, myPeer, _ := runtimeContext.ClusterConfig()
	upgradeManager := runtimeContext.UpgradeManager()
	databaseAutoUpgrade := upgradeManager.ServerDatabaseAutoUpgrade(serverType)
	args, err := createServerArgs(log, config, clusterConfig, myContainerDir, myContainerLogFile, myPeer.ID, myHostAddress, strconv.Itoa(myPort), serverType, arangodConfig,
		containerSecretFileName, bsCfg.RecoveryAgentID, databaseAutoUpgrade, features)
	if err != nil {
		return nil, false, maskAny(err)
	}
	writeCommand(log, filepath.Join(myHostDir, processType.CommandFileName()), config.serverExecutable(processType), args)
	// Collect volumes
	vols := addVolume(confVolumes, myHostDir, myContainerDir, false)
	// Start process/container
	containerNamePrefix := ""
	if config.DockerContainerName != "" {
		containerNamePrefix = fmt.Sprintf("%s-", config.DockerContainerName)
	}
	containerName := fmt.Sprintf("%s%s-%s-%d-%s-%d", containerNamePrefix, serverType, myPeer.ID, restart, myHostAddress, myPort)
	ports := []int{myPort}
	p, err = runner.Start(ctx, processType, args[0], args[1:], createEnvs(config, serverType), vols, ports, containerName, myHostDir, nil)
	if err != nil {
		return nil, false, maskAny(err)
	}
	if databaseAutoUpgrade {
		// Notify the context that we've succesfully started a server with database.auto-upgrade on.
		upgradeManager.ServerDatabaseAutoUpgradeStarter(serverType)
	}
	return p, false, nil
}

// showRecentLogs dumps the most recent log lines of the server of given type to the console.
func (s *runtimeServerManager) showRecentLogs(log zerolog.Logger, runtimeContext runtimeServerManagerContext, serverType definitions.ServerType) {
	logPath, err := runtimeContext.serverHostLogFile(serverType)
	if err != nil {
		log.Error().Err(err).Msg("Cannot find server host log file")
		return
	}
	logFile, err := os.Open(logPath)
	if os.IsNotExist(err) {
		log.Info().Msgf("Log file for %s is empty", serverType)
	} else if err != nil {
		log.Error().Err(err).Msgf("Cannot open log file for %s", serverType)
	} else {
		defer logFile.Close()
		rd := bufio.NewReader(logFile)
		lines := [20]string{}
		maxLines := 0
		for {
			line, err := rd.ReadString('\n')
			if line != "" || err == nil {
				copy(lines[1:], lines[0:])
				lines[0] = line
				if maxLines < len(lines) {
					maxLines++
				}
			}
			if err != nil {
				break
			}
		}
		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprintf("## Start of %s log\n", serverType))
		for i := maxLines - 1; i >= 0; i-- {
			buf.WriteString("\t" + strings.TrimSuffix(lines[i], "\n") + "\n")
		}
		buf.WriteString(fmt.Sprintf("## End of %s log", serverType))
		log.Info().Msg(buf.String())
	}
}

// rotateLogFile rotates the log file of a single server.
func (s *runtimeServerManager) rotateLogFile(ctx context.Context, log zerolog.Logger, runtimeContext runtimeServerManagerContext, myPeer Peer, serverType definitions.ServerType, p Process, filesToKeep int) {
	if p == nil {
		return
	}

	// Prepare log path
	logPath, err := runtimeContext.serverHostLogFile(serverType)
	if err != nil {
		log.Debug().Err(err).Msgf("Failed to get host log file for '%s'", serverType)
		return
	}
	log.Debug().Msgf("Rotating %s log file: %s", serverType, logPath)

	// Move old files
	for i := filesToKeep; i >= 0; i-- {
		var logPathX string
		if i == 0 {
			logPathX = logPath
		} else {
			logPathX = logPath + fmt.Sprintf(".%d", i)
		}
		if _, err := os.Stat(logPathX); err == nil {
			if i == filesToKeep {
				// Remove file
				if err := os.Remove(logPathX); err != nil {
					log.Error().Err(err).Msgf("Failed to remove %s: %s", logPathX)
				} else {
					log.Debug().Msgf("Removed old log file: %s", logPathX)
				}
			} else {
				// Rename log[.i] -> log.i+1
				logPathNext := logPath + fmt.Sprintf(".%d", i+1)
				if err := os.Rename(logPathX, logPathNext); err != nil {
					log.Error().Err(err).Msgf("Failed to move %s to %s", logPathX, logPathNext)
				} else {
					log.Debug().Msgf("Moved log file %s to %s", logPathX, logPathNext)
				}
			}
		}
	}

	// Send HUP signal
	if err := p.Hup(); err != nil {
		log.Error().Err(err).Msg("Failed to send HUP signal")
	}
	return
}

// RotateLogFiles rotates the log files of all servers
func (s *runtimeServerManager) RotateLogFiles(ctx context.Context, log zerolog.Logger, logService logging.Service, runtimeContext runtimeServerManagerContext, config Config) {
	log.Info().Msg("Rotating log files...")
	logService.RotateLogFiles()
	_, myPeer, _ := runtimeContext.ClusterConfig()
	if myPeer == nil {
		log.Error().Msg("Cannot find my own peer in cluster configuration")
	} else {
		if w := s.syncWorkerProc; w != nil {
			if p := w.Process(); p != nil {
				s.rotateLogFile(ctx, log, runtimeContext, *myPeer, definitions.ServerTypeSyncWorker, p, config.LogRotateFilesToKeep)
			}
		}
		if w := s.syncMasterProc; w != nil {
			if p := w.Process(); p != nil {
				s.rotateLogFile(ctx, log, runtimeContext, *myPeer, definitions.ServerTypeSyncMaster, p, config.LogRotateFilesToKeep)
			}
		}
		if w := s.singleProc; w != nil {
			if p := w.Process(); p != nil {
				s.rotateLogFile(ctx, log, runtimeContext, *myPeer, definitions.ServerTypeSingle, p, config.LogRotateFilesToKeep)
			}
		}
		if w := s.coordinatorProc; w != nil {
			if p := w.Process(); p != nil {
				s.rotateLogFile(ctx, log, runtimeContext, *myPeer, definitions.ServerTypeCoordinator, p, config.LogRotateFilesToKeep)
			}
		}
		if w := s.dbserverProc; w != nil {
			if p := w.Process(); p != nil {
				s.rotateLogFile(ctx, log, runtimeContext, *myPeer, definitions.ServerTypeDBServer, p, config.LogRotateFilesToKeep)
			}
		}
		if w := s.agentProc; w != nil {
			if p := w.Process(); p != nil {
				s.rotateLogFile(ctx, log, runtimeContext, *myPeer, definitions.ServerTypeAgent, p, config.LogRotateFilesToKeep)
			}
		}
	}
}

// Run starts all relevant servers and keeps the running.
func (s *runtimeServerManager) Run(ctx context.Context, log zerolog.Logger, runtimeContext runtimeServerManagerContext, runner Runner, config Config, bsCfg BootstrapConfig) {
	defer s.clean()

	_, myPeer, mode := runtimeContext.ClusterConfig()
	if myPeer == nil {
		log.Fatal().Msg("Cannot find my own peer in cluster configuration")
	}

	actions.RegisterAction(&ActionResignLeadership{
		runtimeContext: runtimeContext,
	})

	if mode.IsClusterMode() {
		// Start agent:
		if myPeer.HasAgent() {
			s.agentProc = NewProcessWrapper(s, ctx, log, runtimeContext, runner, config, bsCfg, *myPeer, definitions.ServerTypeAgent, time.Minute)
			time.Sleep(time.Second)
		}

		// Start DBserver:
		if bsCfg.StartDBserver == nil || *bsCfg.StartDBserver {
			s.dbserverProc = NewProcessWrapper(s, ctx, log, runtimeContext, runner, config, bsCfg, *myPeer, definitions.ServerTypeDBServer, time.Minute)
			time.Sleep(time.Second)
		}

		// Start Coordinator:
		if bsCfg.StartCoordinator == nil || *bsCfg.StartCoordinator {
			s.coordinatorProc = NewProcessWrapper(s, ctx, log, runtimeContext, runner, config, bsCfg, *myPeer, definitions.ServerTypeCoordinator, time.Minute)
		}

		// Start sync master
		if bsCfg.StartSyncMaster == nil || *bsCfg.StartSyncMaster {
			s.syncMasterProc = NewProcessWrapper(s, ctx, log, runtimeContext, runner, config, bsCfg, *myPeer, definitions.ServerTypeSyncMaster, time.Minute)
		}

		// Start sync worker
		if bsCfg.StartSyncWorker == nil || *bsCfg.StartSyncWorker {
			s.syncWorkerProc = NewProcessWrapper(s, ctx, log, runtimeContext, runner, config, bsCfg, *myPeer, definitions.ServerTypeSyncWorker, time.Minute)
		}
	} else if mode.IsActiveFailoverMode() {
		// Start agent:
		if myPeer.HasAgent() {
			s.agentProc = NewProcessWrapper(s, ctx, log, runtimeContext, runner, config, bsCfg, *myPeer, definitions.ServerTypeAgent, time.Minute)
			time.Sleep(time.Second)
		}

		// Start Single server:
		if myPeer.HasResilientSingle() {
			s.singleProc = NewProcessWrapper(s, ctx, log, runtimeContext, runner, config, bsCfg, *myPeer, definitions.ServerTypeResilientSingle, time.Minute)
		}
	} else if mode.IsSingleMode() {
		// Start Single server:
		s.singleProc = NewProcessWrapper(s, ctx, log, runtimeContext, runner, config, bsCfg, *myPeer, definitions.ServerTypeSingle, time.Minute)
	}

	// Wait until context is cancelled, then we'll stop
	<-ctx.Done()
	s.stopping = true

	log.Info().Msg("Shutting down services...")
	timeout := getTimeoutProcessTermination(definitions.ServerTypeSyncWorker)
	if p := s.syncWorkerProc; p != nil {
		if !p.Wait(timeout) {
			log.Warn().Str("timeout", timeout.String()).
				Str("type", definitions.ServerTypeSyncWorker).
				Msg("did not terminate in time")
		}
	}

	timeout = getTimeoutProcessTermination(definitions.ServerTypeSyncMaster)
	if p := s.syncMasterProc; p != nil {
		if !p.Wait(timeout) {
			log.Warn().Str("timeout", timeout.String()).
				Str("type", definitions.ServerTypeSyncMaster).
				Msg("did not terminate in time")
		}
	}

	timeout = getTimeoutProcessTermination(definitions.ServerTypeSingle)
	if p := s.singleProc; p != nil {
		if !p.Wait(timeout) {
			log.Warn().Str("timeout", timeout.String()).
				Str("type", definitions.ServerTypeSingle).
				Msg("did not terminate in time")
		}
	}

	timeout = getTimeoutProcessTermination(definitions.ServerTypeCoordinator)
	if p := s.coordinatorProc; p != nil {
		if !p.Wait(timeout) {
			log.Warn().Str("timeout", timeout.String()).
				Str("type", definitions.ServerTypeCoordinator).
				Msg("did not terminate in time")
		}
	}

	timeout = getTimeoutProcessTermination(definitions.ServerTypeDBServer)
	if p := s.dbserverProc; p != nil {
		if !p.Wait(timeout) {
			log.Warn().Str("timeout", timeout.String()).
				Str("type", definitions.ServerTypeDBServer).
				Msg("did not terminate in time")
		}
	}

	timeout = getTimeoutProcessTermination(definitions.ServerTypeAgent)
	if p := s.agentProc; p != nil {
		time.Sleep(3 * time.Second)
		if !p.Wait(timeout) {
			log.Warn().Str("timeout", timeout.String()).
				Str("type", definitions.ServerTypeAgent).
				Msg("did not terminate in time")
		}
	}

	// Cleanup containers
	if w := s.syncWorkerProc; w != nil {
		if p := w.Process(); p != nil {
			if err := p.Cleanup(); err != nil {
				log.Warn().Err(err).Msg("Failed to cleanup sync worker")
			}
		}
	}
	if w := s.syncMasterProc; w != nil {
		if p := w.Process(); p != nil {
			if err := p.Cleanup(); err != nil {
				log.Warn().Err(err).Msg("Failed to cleanup sync master")
			}
		}
	}
	if w := s.singleProc; w != nil {
		if p := w.Process(); p != nil {
			if err := p.Cleanup(); err != nil {
				log.Warn().Err(err).Msg("Failed to cleanup single server")
			}
		}
	}
	if w := s.coordinatorProc; w != nil {
		if p := w.Process(); p != nil {
			if err := p.Cleanup(); err != nil {
				log.Warn().Err(err).Msg("Failed to cleanup coordinator")
			}
		}
	}
	if w := s.dbserverProc; w != nil {
		if p := w.Process(); p != nil {
			if err := p.Cleanup(); err != nil {
				log.Warn().Err(err).Msg("Failed to cleanup dbserver")
			}
		}
	}
	if w := s.agentProc; w != nil {
		if p := w.Process(); p != nil {
			time.Sleep(3 * time.Second)
			if err := p.Cleanup(); err != nil {
				log.Warn().Err(err).Msg("Failed to cleanup agent")
			}
		}
	}

	// Cleanup runner
	if err := runner.Cleanup(); err != nil {
		log.Warn().Err(err).Msg("Failed to cleanup runner: %v")
	}
}

// RestartServer triggers a restart of the server of the given type.
func (s *runtimeServerManager) RestartServer(log zerolog.Logger, serverType definitions.ServerType) error {
	var p Process

	switch serverType {
	case definitions.ServerTypeAgent:
		p = s.agentProc.Process()
	case definitions.ServerTypeDBServer:
		p = s.dbserverProc.Process()
	case definitions.ServerTypeDBServerNoResign:
		p = s.dbserverProc.Process()
	case definitions.ServerTypeCoordinator:
		p = s.coordinatorProc.Process()
	case definitions.ServerTypeSingle, definitions.ServerTypeResilientSingle:
		p = s.singleProc.Process()
	case definitions.ServerTypeSyncMaster:
		p = s.syncMasterProc.Process()
	case definitions.ServerTypeSyncWorker:
		p = s.syncWorkerProc.Process()
	default:
		return maskAny(fmt.Errorf("Unknown server type '%s'", serverType))
	}

	if p != nil {
		terminateProcessWithActions(log, p, serverType, 0, time.Minute, actions.ActionTypeAll)
	}
	return nil
}

// getTimeoutProcessTermination returns how long it should wait for termination for a given server type.
func getTimeoutProcessTermination(serverType definitions.ServerType) time.Duration {
	if serverType == definitions.ServerTypeDBServer {
		return time.Minute * 5
	}

	// return default timeout
	return time.Minute
}
