# Proof-of-Work Blockchain Consensus System

## Overview
This document provides a comprehensive overview of the Proof-of-Work (PoW) blockchain consensus system. The system consists of several components, including User, Worker, and Honest Tracker, which collaborate to maintain a decentralized, secure, and consistent blockchain ledger.

## System Components
The implementation includes three roles: user, worker, and tracker. Users contact workers to post content to the
blockchain and read what other users have posted. Workers gather content from users, mine a new block and attach the contents to the blockchain. A new worker can contact a tracker to obtain the current state of the blockchain, the number of current peer workers, and the port of peer workers.
### User
A user sends an HTTP request to a worker to read or write posts. A user is uniquely identified by its public key.
### Worker
A worker runs automatically and uses HTTP to announce new blocks to its peers. When a new worker wants to join the blockchain it uses HTTP to request information about the current state of the blockchain.
### Tracker
A tracker runs automatically and answers requests from workers.
## Design Requirements and Specifications
This section contains specific details regarding the design and assumptions of our system.
## Assumptions
A worker never lies to a client.
A tracker never lies to a worker.
In other cases anyone can lie to anyone, but most participants in the system are rational and should work for their own best benefits.
### User
- The User component serves as the interface for users to interact with the blockchain system.
- It allows users to read the current state of the blockchain and submit new content through HTTP requests.
- The User component validates and sanitizes the submitted content to ensure data integrity and security.
- After successful submission, the User component sends the content to one or more Worker components for processing.

### Worker
- Worker nodes participate in the block mining process and validate transactions to reach consensus on the state of the blockchain.
- When a new Worker node joins the network, it registers itself with the system, providing its network address and authentication credentials.
- Worker nodes have read and write permissions to act on behalf of the User, allowing them to retrieve information and submit content.
- They obtain nonce values from the nonce host for block mining and can discover and connect to other Worker nodes in the network.
- Worker nodes select a subset of content from the pool, construct new blocks, and perform the proof-of-work algorithm to find valid block hashes.
- Once a valid block is found, the Worker submits the block to the network for validation and acceptance.
- Worker nodes also participate in the validation process by verifying the validity of blocks submitted by other Worker nodes.

### Honest Tracker
- The Honest Tracker is a special type of Worker node that always remains honest and helps maintain the integrity of the blockchain.
- It maintains a separate copy of the blockchain and the content pool.
- When a new Worker node joins the network, it registers itself with the Honest Tracker.
- The Honest Tracker acknowledges the new Worker and adds it to the list of registered Workers.
- It receives broadcasted content and blocks from Worker nodes and validates them against its local copy of the blockchain.
- The Honest Tracker maintains a record of the current length of the blockchain and serves as a reference for the current state of the blockchain.
- When a new Worker node registers, it contacts the Honest Tracker to obtain the current length of the blockchain for synchronization purposes.

## System Workflow

1. User Interaction:
   - Users interact with the blockchain system through the User component.
   - They can read the current state of the blockchain and submit new content using HTTP requests.

2. Content Submission and Broadcasting:
   - When a User submits new content, the User component sends it to one or more Worker nodes for processing.
   - The Worker nodes broadcast the content to other Worker nodes and the Honest Tracker.

3. Block Mining and Broadcasting:
   - Worker nodes participate in the block mining process to create new blocks.
   - They select a subset of content from the pool, construct new blocks, and perform the proof-of-work algorithm to find valid block hashes.
   - When a Worker finds a valid block, it broadcasts the block to the network, including the Honest Tracker.

4. Block Validation and Acceptance:
   - Worker nodes and the Honest Tracker receive broadcasted blocks and validate them against their local copy of the blockchain.
   - If a block is valid, it is appended to the local blockchain, and the blockchain length is updated.

5. Blockchain Synchronization:
   - Worker nodes periodically synchronize their local blockchain copies with the Honest Tracker.
   - They compare the length and hash of their local blockchain with the information provided by the Honest Tracker.
   - If a discrepancy is detected, Worker nodes request missing blocks from the Honest Tracker or other Worker nodes to update their local blockchain copy.

6. Worker Registration and Blockchain Length Information:
   - When a new Worker node joins the network, it registers itself with the system and the Honest Tracker.
   - The new Worker node contacts the Honest Tracker to obtain the current length of the blockchain for synchronization purposes.

## Security and Trust
- The system incorporates security measures, such as authentication and encryption, to protect the communication between nodes.
- The Honest Tracker serves as a trusted entity that helps maintain the integrity and consistency of the blockchain.
- However, relying on a single Honest Tracker introduces a potential point of centralization and trust.
- In a fully decentralized system, the consensus mechanism and the collective behavior of honest nodes should be sufficient to maintain the integrity of the blockchain.

## Scalability and Performance
- The system is designed to scale horizontally by allowing the addition of new Worker nodes to the network.
- The block mining process is optimized to achieve high throughput and low latency.
- The system efficiently handles a large number of content submissions and supports a high rate of block generation.
- The communication protocols between nodes are designed to be efficient and minimize network overhead.

## Programming Language:

We will use Go (Golang) as the primary programming language for implementing the Proof-of-Work blockchain consensus system.
Go provides excellent support for concurrent programming, networking, and cryptography, making it well-suited for blockchain development.

## Third-Party Libraries:

Gin Web Framework: A lightweight and fast web framework for building the HTTP endpoints and handling user requests.

## Test Cases
1. Test whether trackers function normally and a worker can join the blockchain system.
2. Test whether a user can post to a blockchain and can read other user's posts.
3. Test whether the system rejects a block when a worker fakes or replays a user's post.
4. Test whether the system rejects all but one block when multiple valid blocks are submitted at the same time.
5. Test whether a worker can fake a block by "out-computing" its peers.

