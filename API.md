# Specification
## User
1. Ask a tracker for a miner.
2. Read all posts.
3. Write a post.

## Miner
1. Register itself to the tracker and get number of participants and up-to-date blockchain.
2. Miner needs to send heartbeats to the tracker.
3. Miner accepts write requests from users and adds them to its own post pool.
4. Miner syncs post pool with (some of) other known miners.
5. Miner answers read request from a user.
6. Miner mines a new block and broadcasts it to all known miners and trackers. If it receives above 2/3 success votes it
can proceed. Otherwise, it gets the most up-to-date blockchain from the tracker again.
7. Miner needs to answer other miner's broadcasts and updates its blockchain correspondingly.
8. Miner needs to keep track of all known miners. It can know new miners when new miners are broadcasting or syncing pool.

## Tracker
1. Tracker answers a user request with a random miner.
2. Tracker answers register requests from miners and returns a list of all miners and up-to-date blockchain.
3. Tracker answers heartbeats with the current number of miners.

# API
## Tracker
### User requests a miner
**Command**: `/get_miner`

**Method**: `GET`

### A miner registers itself
**Command**: `/register`

**Method**: `POST`

### A miner sends heartbeat
**Command**: `/heartbeat`

**Method**: `GET`

## Miner
### A user sends a read request
**Command**: `/read`

**Method**: `GET`

### A user sends a write request
**Command**: `/write`

**Method**: `POST`

### Another miner syncs with this miner
**Command**: `/sync`

**Method**: `POST`

### Another miner wants to broadcast its new block
**Command**: `/broadcast`

**Method**: `POST`