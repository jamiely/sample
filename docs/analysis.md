Design decisions I made and Why
===============================
* I chose to fan out the query tasks in order to
  make it easier to change the dispatch criteria later.
* I created a SQL connection for each query instead of
  having a single connection per worker. I thought
  that the difference was negligble and that we weren't
  calculating the time it takes to create a connection so
  it would not matter either way. Theoretically we only
  need one connection per worker.

Decisions I made in the interest of time
========================================

* I didn't check input validity
* I assumed the order of CSV fields
* I didn't separate presentation format of the statistics
  from their calculation
* The code I wrote wasn't easily testable. I would try to
  factor out some of the logic to make it more easily
  pluggable for testing.
* I would probably separate the code a little more by logical
  function.