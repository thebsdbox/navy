# Navy

The Navy project is a distributed leaderElection [algorithim](https://en.wikipedia.org/wiki/Bully_algorithm) based upon member learning and fast-failover.

## How it works

### Glossary

This are terms I made up that are obviously going to be naval themed!

- `Admiral`, the leader of the fleet
- `Captain`, a member of the fleet
- `Rank`, the relative importance of a fleet member
- `Fleet`, a collection of members

### Architecture

When a member starts, they can be given the ready flag, meaning that an election will take place (ultimately promoting them to `Admiral`). Once the fleet has been established, other members can then join that fleet as a `Captain` and begin the process of electing a new `Admiral`. The election is performed based upon the rank of a captain, the higher their rank the more likely they are to be promored to the `Admiral`. 

### Joining an existing `Fleet`

A new member can join an existing `Fleet`, simply by connecting to any member of the fleet. Once the ***new** member connects to a fleet a _discover_ process will occur, where the **new** member will be redirected to the leader of the fleet. Once redirected the list of peers is sent to the member and an election process will occur.

## Using as a library

The example `main.go` has largely everything you would need to understand how it works, however the `tl;dr` is that the new captain is passed functions that are executed on `Promotion` and `Demotion`. When the elections take place and one of these events occur, then the function will be called!

### Create a new captain
```go
	b, err := navy.NewCaptain(*rank, *addr, "tcp4", *ready, *fleet, remotePeers)
```

### Set the callback functions

```go
	promotedFunc := func() {
		log.Info("Im the Admiral")

	}
	demotionFunc := func() {
		log.Info("I've been demoted to Captain'")

	}

	b.OnPromotion(promotedFunc)
	b.OnDemotion(demotionFunc)
```
### Start the membership!

```go
	b.Run(nil)
```
