---
page_title: "seaweedfs_iam_group_policy_attachment Resource - seaweedfs"
description: Manages one IAM group policy attachment.
---

# seaweedfs_iam_group_policy_attachment

Manages one managed-policy attachment to a group. Existing attachments must be imported before they can be managed; deletion detaches only this policy.

- `group_name` (required): group name.
- `policy_arn` (required): managed policy ARN.
- `id` (read-only): `group_name|policy_arn`.

Import using `group_name|policy_arn`, quoted in the shell to avoid interpreting `|` as a pipe.
