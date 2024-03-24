![Test](https://github.com/shogo82148/schemalex-deploy/workflows/Test/badge.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/shogo82148/schemalex-deploy.svg)](https://pkg.go.dev/github.com/shogo82148/schemalex-deploy)
[![Coverage Status](https://coveralls.io/repos/github/shogo82148/schemalex-deploy/badge.svg?branch=main)](https://coveralls.io/github/shogo82148/schemalex-deploy?branch=main)

# schemalex-deploy

database migration tool for MySQL.

## SYNOPSIS

You want to deploy the following SQL schema:

```sql
-- schema.sql
CREATE TABLE hoge (
    id INTEGER NOT NULL AUTO_INCREMENT,
    PRIMARY KEY (id)
);
```

You can use schemalex-deploy to actually reflect it in the MySQL database.

```plain
$ schemalex-deploy -host 127.0.0.1 -port 3306 -user root -password password -database gotest schema.sql
CREATE TABLE `hoge` (
  `id` INT (11) NOT NULL AUTO_INCREMENT,
  PRIMARY KEY (`id`)
);
Do you want to perform these actions?
Only 'yes' will be accepted to confirm.
Enter a value: yes
2024/03/24 22:48:00 starting to deploy
2024/03/24 22:48:00 executing: CREATE TABLE `hoge` (
  `id` INT (11) NOT NULL AUTO_INCREMENT,
  PRIMARY KEY (`id`)
)
2024/03/24 22:48:00 updating the schema information
2024/03/24 22:48:00 done
```

Next, you wanted to update the schema as follows.

```sql
-- schema.sql
CREATE TABLE hoge (
    id INTEGER NOT NULL AUTO_INCREMENT,
    c VARCHAR (20) NOT NULL DEFAULT "hoge",
    PRIMARY KEY (id)
);

CREATE TABLE fuga (
    id INTEGER NOT NULL AUTO_INCREMENT,
    PRIMARY KEY (id)
);
```

To do this, you need to construct the necessary SQL, such as ALTER statements, for the update.
If you use schemalex-deploy, it can automatically calculate the necessary SQL statements and actually reflect them.

```plain
$ schemalex-deploy -host 127.0.0.1 -port 3306 -user root -password password -database gotest schema.sql
2024/03/24 22:50:34 import table: hoge
2024/03/24 22:50:34 import table: schemalex_revision
CREATE TABLE `fuga` (
  `id` INT (11) NOT NULL AUTO_INCREMENT,
  PRIMARY KEY (`id`)
);
ALTER TABLE `hoge` ADD COLUMN `c` VARCHAR (20) NOT NULL DEFAULT 'hoge' AFTER `id`;
Do you want to perform these actions?
Only 'yes' will be accepted to confirm.
Enter a value: yes
2024/03/24 22:50:44 starting to deploy
2024/03/24 22:50:44 executing: CREATE TABLE `fuga` (
  `id` INT (11) NOT NULL AUTO_INCREMENT,
  PRIMARY KEY (`id`)
)
2024/03/24 22:50:44 executing: ALTER TABLE `hoge` ADD COLUMN `c` VARCHAR (20) NOT NULL DEFAULT 'hoge' AFTER `id`
2024/03/24 22:50:44 updating the schema information
2024/03/24 22:50:44 done
```

## COMMAND LINE OPTIONS

```
-socket           the unix domain socket path for the database
-host             the host name of the database
-port             the port number(default: 3306)
-user             username
-password         password
-database         the database name
-version          show the version
-auto-approve     skips interactive approval of plan before deploying
-dry-run          outputs the schema difference, and then exit the program
-import           imports existing table schemas from running database
```

## SEE ALSO

- http://blog.gopheracademy.com/advent-2014/parsers-lexers/
- https://github.com/soh335/git-schemalex
- https://github.com/schemalex/schemalex

## LICENSE

MIT
