#Dockdash

Realtime docker ps and stats viewer.

Updates based on docker events and stats streams.

Built in go.

<img src="./screencap.png" alt="Screen grab" width="600">

Use arrow keys to jump between data and traverse container list.

W.I.P right now, please let me know if there's anything you think I should add to this.

#Getting Started

Check out the [releases](http://github.com/byrnedo/dockdash/releases) page to get binaries. 

If you want for a specific arch just raise a ticket.

Otherwise you can build from source (assumes you have `go` installed):
    
    go get github.com/byrnedo/dockdash

##Todo
1. Clean up code.
2. ~~Add more viewable data (ENVs, Entrypoint, Command)~~
3. Batch draw requests
4. Handle multiline info somehow
5. Dockerfile
6. Deb package

#PS
If judging code quality please be gentle, I plan to remove a lot of the unnecessary channels
and break apart the code a little more.
