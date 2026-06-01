---
name: ab-db
description: Connect to the andbegin-monorepo local or UAT MySQL database via the mysql 8.4 client, reading connection details from the codebase.
---

# ab-db

Open a `mysql` session against the `andbegin-monorepo` database — **local** dev
or **UAT** — and run queries on the user's behalf.

The monorepo lives at `~/Code/andbegin-monorepo`. All paths below are relative
to that root.

## 1. Validate the client (always first)

```sh
mysql --version
```

This codebase targets **MySQL 8.4**. Confirm the resolved client reports
`8.4.x`. Note: a separate `mysql` server formula (9.x) may also be installed, so
don't assume — check the version string.

- If it's not 8.4, prefer the explicit Homebrew client:
  `/opt/homebrew/opt/mysql-client@8.4/bin/mysql` (formula `mysql-client@8.4`).
- If neither is present, tell the user to `brew install mysql-client@8.4` and
  **stop** — do not connect with a mismatched client.

## 2. Pick the target

Default to **local** unless the user says UAT (or names a feature environment).

## 3. Local connection

Read the values, then connect:

- **Host / port / password:** `ops/docker-compose.yml`, service `db`.
- **Database name:** defaults to `andbegin`. Confirm from the root `Makefile`
  (`DB_NAME ?= andbegin`) / `backend/data/src/index.ts` (`DB_URL` fallback)
  rather than assuming. All tenants share this one database — tenant separation
  is application-level, so no schema/tenant choice is needed to connect.

Check the container is up before connecting:

```sh
docker compose -p andbegin -f ops/docker-compose.yml ps db
```

**If it's down, tell the user to run `make _start-db` (from the repo root) and
stop. Do not auto-start it.**

Connect (substitute the values read above):

```sh
mysql -h 127.0.0.1 -P <port> -u root -p<password> <db>
```

## 4. UAT connection

UAT uses a **dynamically named** per-environment database. The connection
string is `deployment/uat/dist/ab-mono.<env>.env`, `DB_URL`.

If the file does not exist, prompt the user to run `make env-release`.

**Derive `<env>`:** it will be the only `.env` file in `dist`.

Default the env to the current git branch (`git rev-parse --abbrev-ref HEAD`)
run through that rule, unless the user supplies one.

Parse `user`, `password`, `host`, and `database` out of the `DB_URL`. Do not
hardcode the password — read it from the file.

Connect:

```sh
mysql -h db.dev-app.intern.andbegin.com -u admin -p<password> andbegin_<env>
```

## 5. Running queries

Full access — run read or write queries as the user asks, on either
environment, without extra confirmation prompts.

Be aware UAT is a **shared** environment, but typically only used by one
depeveloper at a time. There is no need to be overly cautious here.

For non-interactive one-off queries, use `-e`:

```sh
mysql -h <host> -P <port> -u <user> -p<password> <db> -e "SELECT 1;"
```
