#!/bin/bash

cd ..
rm navy;go build -o navy examples/server/simpleServer.go 
cd -

export LOG=5
export ADDRESS=192.168.0.22
#tmux \
#    new-session  '../navy -address 0.0.0.0:9990 -rank 100 -ready' \; \
#    split-window 'sleep 2; ../navy -address 0.0.0.0:9991 -rank 80 -raddress 0.0.0.0:9990' \; \
#    split-window 'sleep 3; ../navy -address 0.0.0.0:9992 -id 50 -raddress 0.0.0.0:9990' \; \
#    select-layout even-vertical \;  \
#    detach-client

tmux \
    new-session  '../navy -address $ADDRESS:9990 -rank 100 -ready -log $LOG' \; \
    split-window 'sleep 2; ../navy -address $ADDRESS:9991 -rank 80 -fleet $ADDRESS:9990 -log $LOG' \; \
    split-window 'sleep 3; ../navy -address $ADDRESS:9992 -rank 50 -fleet $ADDRESS:9991 -log $LOG' \; \
    split-window 'sleep 5; ../navy -address $ADDRESS:9993 -rank 101 -fleet $ADDRESS:9992 -log $LOG' \; \
    select-layout even-vertical \;  \
#    detach-client