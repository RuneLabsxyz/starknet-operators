# High level design

As starknet nodes needs to keep state, and it cannot be shared yet,
We need to have two objects:
- StarknetRPCNode
- StarknetRPCDeployment

The Node is a single element, that can either be restored from:
- A Pathfinder archive
- A snapshot of another RPC node (Using a custom backup script)
- A backup made to S3 of the RPC node (for disaster recovery, and made with the backup script)

The Deployment manages N StarknetRPCNodes, handling HA-related features like:
- Deploying multiple instances of the RPC
- Managing Services so that only alive nodes can access them
- Handling live rolling updates (in place, or with alive guarantees)
- Re-scheduling of instances
