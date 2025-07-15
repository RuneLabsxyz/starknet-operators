# Here is how the reconciliation loop works

- Does the PVC exist?
  - If the PVC does not exist:
    - create it with the correct size
  - If it exists:
    - check if the size is correct
    - if not, update the size
- If the node has a snapshot setup:
  - Check if the PVC is annotated with "pathfinder.runelabs.xyz/snapshot-loaded"
  - If not, check if a job is owned by the node, and is still running
  - If it has fi
  - If not, create the
