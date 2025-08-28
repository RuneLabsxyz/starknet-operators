General code improvements to do in order to make it easier to maintain:
- Store condition inside of an easier to understand package (with enums and better functions)
- Split further the controllers into reconciliers (especially for archive), to make it clearer
- Add tests
- Handle situation where the PVC was deleted (so a new restore is needed)
  - Add a new label inside of the PVC to get the info about the job
- Create a service for the RPC node
- Add a monitoring config (using PodMonitor)
- Setup a test dashboard
