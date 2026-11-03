---
page_title: "seaweedfs_iam_group_membership Resource - seaweedfs"
description: Manages one IAM group membership.
---

# seaweedfs_iam_group_membership

Manages one user-to-group relationship. Existing memberships must be imported before they can be managed; deletion removes only this membership.

- `group_name` (required): group name.
- `user_name` (required): user name.
- `id` (read-only): `group_name/user_name`.

Import with `terraform import seaweedfs_iam_group_membership.example group/user`.
