#!/bin/bash

cd ..
rm navy;go build 
cd -

export LOG=4

#tmux \
#    new-session  '../navy -address 0.0.0.0:9990 -rank 100 -ready' \; \
#    split-window 'sleep 2; ../navy -address 0.0.0.0:9991 -rank 80 -raddress 0.0.0.0:9990' \; \
#    split-window 'sleep 3; ../navy -address 0.0.0.0:9992 -id 50 -raddress 0.0.0.0:9990' \; \
#    select-layout even-vertical \;  \
#    detach-client

tmux \
    new-session  '../navy -address 0.0.0.0:9990 -rank 100 -ready -log $LOG' \; \
    split-window 'sleep 2; ../navy -address 0.0.0.0:9991 -rank 80 -fleet 0.0.0.0:9990 -log $LOG' \; \
    split-window 'sleep 3; ../navy -address 0.0.0.0:9992 -rank 50 -fleet 0.0.0.0:9991 -log $LOG' \; \
    split-window 'sleep 5; ../navy -address 0.0.0.0:9993 -rank 101 -fleet 0.0.0.0:9992 -log $LOG' \; \
    select-layout even-vertical \;  \
#    detach-client