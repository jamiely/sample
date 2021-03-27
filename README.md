Intro
=====

This is a db benchmarker.

* `data` - Sample data to use with the project
* `docs` - Documentation
* `schema` - Schemas for the project.

Setup
=====

Requires docker-compose.

```
docker-compose up -d
# Wait for the database to come up.
docker-compose ps timescaledb

# run using the default params file
docker-compose run benchmark -workers 10

# or use stdin
<./data/query_params.csv docker-compose run benchmark -workers 10 -params-file -
```
