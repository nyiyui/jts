# database sync

## blueprint

When initially downloading the database from the server, keep a "original copy" that is not edited.

When syncing:
1. lock the server's database
2. download the server's database
3. perform a 3-way merge (original copy, local copy, server's copy)
4. resolve conflicts (potentially asking user)
5. upload the new database to the server
6. unlock the server's database
7. reset the local database to the new database
8. set a new original copy
