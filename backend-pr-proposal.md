# Proposed PR: Fix 500 Error on CLI Repository Link Endpoint (`/api/v1/cli/repos/link`)

## Description
This PR resolves an issue where the `/api/v1/cli/repos/link` endpoint throws a `500 Internal Server Error` when the Proofboard CLI attempts to link a repository.

## Bug Details
Currently, when the CLI sends the exact payload specified in the v1.8.1 contract to the link endpoint, the server crashes or throws an unhandled exception.

**Request Payload sent by CLI:**
```json
{
  "orgHash": "ce2157eaa8c5c48e8f07ccd5d7903868bdbff9656e99afc6ce40833a008d3a46",
  "repoHash": "d46f0dd66c09a90a867f8717d68f384b8f5d460f0ef0f0ed57c8a27a9246dc8a",
  "provider": "github"
}
```

**Observed Response:**
```json
{"statusCode":500,"timestamp":"...","path":"/api/v1/cli/repos/link","message":"Internal server error"}
```

## Root Cause Analysis
The issue is likely caused by one of the following in the backend route controller for `/api/v1/cli/repos/link`:
1. **Missing Schema Validation / Null Pointer:** The controller might be expecting fields that were removed or renamed in the updated spec (e.g., expecting `repositoryId` instead of `repoHash`) and crashing when they are absent.
2. **Database Constraint Violation:** The backend might be attempting to insert the `repoHash` into a database table without properly catching a unique constraint violation or missing foreign key (like an unknown `orgHash`).
3. **Provider Enum Mismatch:** The database or ORM might be rejecting the `"github"` string if it expects uppercase (`GITHUB`) or a different enum format.

## Proposed Changes
1. **Add Validation:** Ensure the DTO/schema validator strictly accepts `orgHash`, `repoHash`, and `provider` as `string` values, catching validation errors and returning a `400 Bad Request` rather than throwing a `500`.
2. **Fix Controller Logic:** Update the endpoint's business logic to properly check if the repository is already linked. 
   - If it is, return: `{ "isNewProject": false, "projectId": "<id>" }`
   - If it is not, return: `{ "isNewProject": true, "setupUrl": "<url>", "deviceCode": "..." }`
3. **Add Error Boundaries:** Wrap the database queries in a try/catch block and handle edge cases gracefully.
4. **Integration Tests:** Add tests to the backend repository that simulate this exact payload from the CLI.

## Testing Instructions
1. Check out this branch locally.
2. Run the backend server.
3. Send the following curl command:
   ```bash
   curl -X POST http://localhost:8080/api/v1/cli/repos/link \
     -H "Content-Type: application/json" \
     -d '{"orgHash": "ce21...", "repoHash": "d46f...", "provider": "github"}'
   ```
4. Verify that the response is `200 OK` with the appropriate project setup status.
