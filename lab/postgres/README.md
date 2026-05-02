# PostgreSQL control-store integration lab

The disposable database uses Docker Official Image `postgres:18.6-bookworm`
pinned to the multi-platform OCI index digest in `images.json`. The dated suite
qualifier prevents an unnoticed Debian suite change. The local platform
manifest and resulting image ID are recorded by each run.

The harness publishes PostgreSQL only on a dynamic `127.0.0.1` port, stores its
data in a bounded tmpfs, and creates a random password in a mode-0600 temporary
file mounted through `POSTGRES_PASSWORD_FILE`. The password is not placed in
Git, command arguments, Docker environment, or the result report. The database
connection uses `sslmode=disable` only across this loopback-only disposable lab;
this is not the production control-plane TLS design.

The test applies the embedded migration twice, proves identical batch retries
are idempotent, proves a conflict rolls back an earlier insert in the same
batch, and races eight identical deliveries against the unique host/event key.
It tears down only the container whose random name, ID, and Watchhouse label it
created.

Official sources: [Docker Official Image packaging](https://github.com/docker-library/postgres),
[official-images manifest](https://github.com/docker-library/official-images/blob/master/library/postgres),
and [PostgreSQL documentation](https://www.postgresql.org/docs/18/).
