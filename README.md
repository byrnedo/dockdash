#Dockdash

Realtime docker ps and stats viewer.

Updates based on docker events and stats streams.

Built in go.

Ready to roll container available: `docker pull byrnedo/dockdash`

<img src="./screencap.png" alt="Screen grab" width="600">

Use arrow keys to jump between data and traverse container list.

'i' key switches to inspect mode, view multiline data for one container.

W.I.P right now, please let me know if there's anything you think I should add to this.

#Getting Started

Try it out first (requires docker...)

    docker run -it -v /var/run/docker.sock:/var/run/docker.sock byrnedo/dockdash

This will mount /var/run/docker.sock straight into the container :)

Check out the [releases](http://github.com/byrnedo/dockdash/releases) page to get binaries. 

If you want for a specific arch just raise a ticket.

Otherwise you can build yourself (requires docker):

    make build

Output binary will be in `build/`
    

##Todo
1. ~~Clean up code.~~
2. ~~Add more viewable data (ENVs, Entrypoint, Command)~~
3. ~~Batch draw requests~~
4. Handle multiline info somehow
5. ~~Dockerfile~~
6. Deb package

#PS
If judging code quality please be gentle, I plan to remove a lot of the unnecessary channels
and break apart the code a little more.
