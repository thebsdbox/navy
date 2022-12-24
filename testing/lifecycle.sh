#!/bin/bash

cd ..
rm navy;go build 
cd -
rm /tmp/navy

../navy -address 0.0.0.0:9990 -rank 100 -ready &
member1=$!
sleep 2
../navy -address 0.0.0.0:9991 -rank 80 -fleet 0.0.0.0:9990 &
member2=$!
sleep 2
../navy -address 0.0.0.0:9992 -rank 50 -fleet 0.0.0.0:9991 &
member3=$!
sleep 3
../navy -address 0.0.0.0:9999 -rank 800 -fleet 0.0.0.0:9992 &
member4=$!
../navy -address 0.0.0.0:9998 -rank 9000 -fleet 0.0.0.0:9991 -callsign wrong &
sleep 2
echo "Started four members $member1 $member2 $member3 $member4"
kill $member4
sleep 2
kill $member1
sleep 2
kill $member2
sleep 2
kill $member3
sleep 1
cat /tmp/navy
