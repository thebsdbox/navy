#!/bin/bash
EXPECTED="-> 100\n
-> 800\n
<- 100\n
<- 800\n
<- 100\n
-> 80\n
<- 80\n
-> 50\n
<- 50"
echo -e $EXPECTED
cd ..
rm navy;go build -o navy examples/server/simpleServer.go 
cd -
rm /tmp/navy
export LOG=5

../navy -address 0.0.0.0:9990 -rank 100 -ready -log $LOG &
member1=$!
sleep 2
../navy -address 0.0.0.0:9991 -rank 80 -fleet 0.0.0.0:9990 -log $LOG &
member2=$!
sleep 2
../navy -address 0.0.0.0:9992 -rank 50 -fleet 0.0.0.0:9991 -log $LOG &
member3=$!
sleep 3
../navy -address 0.0.0.0:9999 -rank 800 -fleet 0.0.0.0:9992 -log $LOG &
member4=$!
../navy -address 0.0.0.0:9998 -rank 9000 -fleet 0.0.0.0:9991 -callsign wrong -log $LOG &
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
diff -u <(echo $EXPECTED) <(cat /tmp/navy)

