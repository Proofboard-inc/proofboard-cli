# Authentication and Sync Error Report

This report captures the API request and response data during the `link` and `sync` steps of the Proofboard CLI. The CLI is throwing errors when trying to communicate with `api-dev.proofboard.io`.

## 1. `proofboard link` Error

**Command Output:**
```
Detected organisation: Proofboard-inc

--- HTTP POST https://api-dev.proofboard.io/api/v1/cli/repos/link ---
REQUEST:
{
  "orgHash": "ce2157eaa8c5c48e8f07ccd5d7903868bdbff9656e99afc6ce40833a008d3a46",
  "repoHash": "d46f0dd66c09a90a867f8717d68f384b8f5d460f0ef0f0ed57c8a27a9246dc8a",
  "provider": "github"
}

RESPONSE (500 Internal Server Error):
{"statusCode":500,"timestamp":"2026-07-11T10:54:59.784Z","path":"/api/v1/cli/repos/link","message":"Internal server error"}
--------------------

register linked repository: API returned 500 Internal Server Error: {"statusCode":500,"timestamp":"2026-07-11T10:54:59.784Z","path":"/api/v1/cli/repos/link","message":"Internal server error"}
```

**Observation:**
The payload sent for linking is structurally correct as per the `v1.8.1` spec (`orgHash`, `repoHash`, `provider`). However, the backend responds with a `500 Internal Server Error` instead of the expected project object or setup flow branch.

---

## 2. `proofboard sync` Error

**Command Output:**
```
--- HTTP POST https://api-dev.proofboard.io/api/v1/cli/sync ---
REQUEST:
{
  "shas": [ ... ],
  "timestamps": [ ... ],
  "additions": [ ... ],
  "deletions": [ ... ],
  "filesChanged": [ ... ],
  "categories": [ ... ],
  "impactScores": {
    "bugfix": 0,
    "feature": 0.42857142857142855,
    "maintenance": 0,
    "refactor": 0,
    "ship": 0.5714285714285714
  },
  "milestoneClusters": [ ... ],
  "orgHash": "ce2157eaa8c5c48e8f07ccd5d7903868bdbff9656e99afc6ce40833a008d3a46",
  "repoHash": "d46f0dd66c09a90a867f8717d68f384b8f5d460f0ef0f0ed57c8a27a9246dc8a",
  "emailHash": "62a8e090b1517753c6ce498f2184b4a2f6184bd4eed1bc6a394d88633e32e524",
  "provider": "github",
  "capturedAt": "2026-07-11T10:55:05Z",
  "cliVersion": "1.8.1",
  "dictionaryVersion": "1.2.0",
  "antiFraudSignals": {
    "aiNoiseScore": 0.23571428571428582,
    "orgHashMismatch": true,
    "identityMismatch": 0,
    "lowCommitCount": false,
    "singleCommitRepoCap": false
  },
  "notifyPush": false
}

RESPONSE (400 Bad Request):
{"statusCode":400,"timestamp":"2026-07-11T10:55:05.802Z","path":"/api/v1/cli/sync","message":"No linked project found for this repository. Run: proofboard link"}
--------------------

transmit sync payload: API returned 400 Bad Request: {"statusCode":400,"timestamp":"2026-07-11T10:55:05.802Z","path":"/api/v1/cli/sync","message":"No linked project found for this repository. Run: proofboard link"}
```

**Observation:**
The sync command correctly builds the comprehensive JSON payload as required. However, because the repository failed to link previously due to the backend `500` error, the backend rejects the sync operation with a `400 Bad Request` since it lacks an established association.

---
**Conclusion for Backend Team:**
The CLI's local processing and hashing engine function correctly. The `link` payload format is also matching the spec, but it is currently returning a 500 status code from the `/api/v1/cli/repos/link` route on `api-dev.proofboard.io`. Once that resolves, `sync` should seamlessly proceed.
