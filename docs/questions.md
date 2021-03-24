Questions
=========

* Should I use external modules or avoid them? I'm planning on
  using `github.com/jackc/pgx/v4` since it's mentioned in the docs.
* Should I use one connection for each worker? A shared pool 
  for all? One single connection?
* Certain queries may return larger results than others. Do we
  want to somehow normalize?
* The order of queries might matter due to things like caching. 
  Should we perform some form of canonicalization to enforce
  a specific order to the queries? Or randomize the queries?
* Do we care about overlapping queries/duplicate queries? This
  may have some caching impact as well.
* How do you want to handle invalid inputs? I would default to
  outputing the line number (perhaps the params themselves),
  and having a counter of them.
* If there is somehow some query failure, how should it count
  in the statistics? (Not really sure how this could happen
  other than some connection interruption.) I would default to
  having some sort of query failure count and excluding those
  from the other stats.
* You want the database loaded with your sample data as part
  of the assignment? I want to store it in the repository since
  it is quicker but normally I wouldn't. I could pull it from
  Dropbox if you like.
* What do you think of these additional metrics?
    * Percentiles: 99p, 95p, 90p, 50p
    * Std deviation
* It sounds like you want to time_bucket results. Is this what
  you were thinking? For the set of query params below:
  ```csv
  host_000008,2017-01-01 08:59:22,2017-01-01 09:59:22
  ```

  The query looks like:
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

Comments
========
* Seems very dependent on the machine that is running the program.
* When running on a single machine, IO saturation may color 
  results.
* I was thinking about the number of cores on the client mattering
  but probably not b/c we'd be IO-bound.
