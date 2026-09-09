# Security policy

ICMN is pre-release software and must not yet be used as a production identity registry.

The system-level risks, abuse cases, required controls, and release gates are documented in [the ICMN threat assessment](docs/threat-model.md). Changes to trust boundaries, matching, merge/split behavior, connectors, authorization, tenancy, or sensitive data handling must update that assessment in the same pull request.

Please do not disclose vulnerabilities in public issues. Use GitHub's private vulnerability reporting for this repository. Include affected versions, reproduction steps, impact, and any suggested mitigation. Do not include real personal or enterprise master data.

Supported versions will be listed here after the first release. Security-sensitive design areas include identity poisoning, merge/split authorization, tenant isolation, connector credentials, assertion-level access control, audit integrity, semantic laundering, bulk correlation, and personal-data erasure.
