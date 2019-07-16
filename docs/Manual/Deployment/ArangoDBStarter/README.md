Deploying using the ArangoDB Starter
====================================

**Single Instance:**

- [_Starter_ using processes](../SingleInstance/UsingTheStarter.md)
- [_Starter_ using Docker containers](../SingleInstance/UsingTheStarter.md#using-the-arangodb-starter-in-docker)

**Active Failover:**

- [_Starter_ using processes](../ActiveFailover/UsingTheStarter.md)
- [_Starter_ using Docker containers](../ActiveFailover/UsingTheStarter.md#using-the-arangodb-starter-in-docker)

**Cluster:**

- [_Starter_ using processes](../Cluster/UsingTheStarter.md)
- [_Starter_ using Docker containers](../Cluster/UsingTheStarter.md#using-the-arangodb-starter-in-docker)

Best Practices
--------------

### Linux user used to run _arangodb_

It is strongly suggested to not run the _ArangoDB Starter_ process (_arangodb_)
as _root_. We suggest creating a dedicated Linux user for this purpose.

Assuming the user you want to use is called _arangodb_, to run the _Starter_,
on Linux systems, you might use a command similar to the following:

```
sudo su arangodb -s /bin/bash -c "arangodb --all.server.uid=$(id -u arangodb) ... AnyStarterOptionHere"
```
