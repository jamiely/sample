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
# run using the default params file
docker-compose run benchmark -workers 10
# or use stdin
<./data/query_params.csv docker-compose run benchmark -workers 10 -params-file -
```
