# GORM Dynamic Authentication

This package implements easy-to-use dynamic authentication for GORM connections. This is useful when using any kind of authentication mechanism where credentials may change between when the GORM handle is initially created and when new connections may open. Examples could include username/password credentials that are rotated frequently, or IAM authentication with AWS RDS.


## Database Support

This package supports MySQL, and theoretically supports PostgreSQL as well, although that has not yet been tested. It is built in a modular fashion that supports the implementation of additional databases as well. See the `connectors` and `dialectors` submodules.


## Examples

We have provided examples for the following use cases:

- [Generic username/password MySQL](https://github.com/Invicton-Labs/gorm-auth/blob/main/examples/aws-rds-mysql-password-auth.go)
- [AWS RDS IAM authentication for MySQL](https://github.com/Invicton-Labs/gorm-auth/blob/main/examples/aws-rds-mysql-iam-auth.go).

For more custom implementations (multiple sources, multiple replicas, etc.), see the internal workings of the functions used in the examples.
