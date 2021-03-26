Intro
=====

This is a db benchmarker.

* `data` - Sample data to use with the project
* `docs` - Documentation
* `schema` - Schemas for the project.

Setup
=====

```
docker-compose up -d
docker-compose exec timescaledb psql -U postgres -d homework -c "\COPY cpu_usage FROM /working/data/cpu_usage.csv CSV HEADER"
```

Links
=====

* https://docs.timescale.com/latest/tutorials/quickstart-go

Investigation
=============

```sql
select 
    host, 
    time_bucket('1 minute', ts) as onemin, 
    min(usage), 
    max(usage) 
    from cpu_usage 
    where host = 'host_000008' 
    group by host, onemin 
    order by onemin;
```

```
    host     |         onemin         |  min  |  max
-------------+------------------------+-------+-------
 host_000008 | 2017-01-01 00:00:00+00 |   3.1 | 66.25
 host_000008 | 2017-01-01 00:01:00+00 |  1.84 | 89.94
 host_000008 | 2017-01-01 00:02:00+00 | 21.19 | 84.66
 host_000008 | 2017-01-01 00:03:00+00 | 10.94 | 98.64
 host_000008 | 2017-01-01 00:04:00+00 |  5.34 | 91.55
 host_000008 | 2017-01-01 00:05:00+00 |   1.7 | 81.14
 host_000008 | 2017-01-01 00:06:00+00 |  6.67 | 86.38
 host_000008 | 2017-01-01 00:07:00+00 |  19.1 | 79.28
 host_000008 | 2017-01-01 00:08:00+00 | 15.83 | 40.58
 host_000008 | 2017-01-01 00:09:00+00 |  6.46 |  93.9
 host_000008 | 2017-01-01 00:10:00+00 | 24.23 | 84.92
 host_000008 | 2017-01-01 00:11:00+00 |  9.56 | 77.05
 host_000008 | 2017-01-01 00:12:00+00 | 16.34 | 95.69
```

Example
=======

query params:

```
host_000008,2017-01-01 08:59:22,2017-01-01 09:59:22
```

```sql
select 
    host, 
    time_bucket_gapfill('1 minute', ts) as onemin, 
    min(usage), 
    max(usage) 
    from cpu_usage 
    where 
        host = 'host_000008'
        and ts between '2017-01-01 08:59:22' 
                   and '2017-01-01 09:59:22'
    group by host, onemin 
    order by onemin;
```

Build
=====

```bash
go get github.com/jackc/pgx/v4
```