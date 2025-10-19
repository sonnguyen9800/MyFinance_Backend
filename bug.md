# Browser Signup Failure Analysis

## Summary
The Flutter mobile app could sign users up without issue, but the new Flutter web build showed `DioException [connection error]: The XMLHttpRequest onError callback was called`. The backend rejected the request before it reached the Go/Gin handlers.

## Root Cause
Gin was serving the API without any Cross-Origin Resource Sharing (CORS) headers. When the web client sent `POST /api/signup`, the browser enforced the *same-origin policy*. Because the web app and API lived on different origins, the browser looked for explicit permission via `Access-Control-Allow-*` headers. None were present, so the browser blocked the request and raised an `XMLHttpRequest` network error. The failure occurred entirely in the browser; the server never handled the signup request.

## Why Mobile Worked
Native mobile apps (and the Android emulator) use the device networking stack, not the browser. These stacks do **not** implement the browser's same-origin policy or CORS checks. As long as the device can reach the API URL, the HTTP request succeeds. The same API endpoint therefore worked on mobile while being blocked in browsers.

## CORS in a Nutshell
- **Same-Origin Policy (SOP)**: Browsers isolate scripts, allowing them to read responses only if the request target shares the same origin (scheme + host + port) as the page. This mitigates cross-site data leaks.
- **Cross-Origin Resource Sharing (CORS)**: A standard that lets servers opt into sharing resources with other origins. When a web page makes a cross-origin request, the browser checks the response for headers like `Access-Control-Allow-Origin`. Without them the browser aborts the request.
- **Preflight Requests**: For mutating methods (POST/PUT/DELETE) or custom headers, browsers send an `OPTIONS` preflight call first. The server must respond with the permitted origins, methods, and headers.

## Fix Direction
Add CORS middleware to Gin before registering routes (for example, `github.com/gin-contrib/cors`) and allow the web app's origin, relevant methods (`GET`, `POST`, etc.), and headers (`Authorization`, `Content-Type`). Alternatively serve the Flutter web build and API from the same origin. Once the server returns the appropriate CORS headers, browsers accept the response and signup proceeds without the `XMLHttpRequest` error.
