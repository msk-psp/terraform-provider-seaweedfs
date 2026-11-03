---
page_title: "seaweedfs_iam_group Resource - seaweedfs"
description: Manages a SeaweedFS IAM group.
---

# seaweedfs_iam_group

Manages a group. Deletion is refused if it has members, attached managed policies, or inline policies. The IAM endpoint must support listing these relationships.

- `name` (required): group name; changing it replaces the resource.
- `id` (read-only): group name.

Import using the group name, for example `terraform import seaweedfs_iam_group.readers readers`.
