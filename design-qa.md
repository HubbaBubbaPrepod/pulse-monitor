# Design QA — Pulse Monitor

**Result: passed for the frontend demo scope.**

## Reference and rendering

Compared the selected monochrome overview concept and the implemented page together in `artifacts/comparison.png`. The implementation preserves the PULSE sidebar, server identity strip, four metric cards, load chart, resource donut, process table and log feed. Uses neutral grayscale only. The white theme reverses the visual hierarchy while keeping labels and chart lines readable.

Intentional differences: functional line icons replace rendered rack art; an explicit demo label and server picker are present; spacing is roomier than the generated reference. This is a working adaptation, not a claim of pixel-identical reproduction.

## Initial frontend checks (0.1.0)

- Desktop layout at 1584 × 992; dark and light full-page captures saved.
- Mobile at 390 × 844: navigation opens, drawer fits the screen, and the document has no horizontal overflow (375 px client/scroll width, excluding scrollbar).
- Theme and compact preference survive reload.
- Add-server validation rejects an empty name; a test server can be added and removed. Final list restored to the three initial demo servers.
- Offline server displays an unavailable state without current metrics.
- Windows server displays Windows process names.
- Process search returns the matching process; ERROR log filtering and log pause work.
- Chart period buttons change selection and the data table opens/closes.
- All events can be acknowledged; unread filter then displays the empty state.
- Motion switch works; CSS and GSAP honor reduced motion. System reduced-motion emulation was not separately tested.
- Ctrl+K opens quick navigation and server search works.
- CSV download completed in the browser.
- No browser console errors observed. Initial GSAP warnings for absent panels during skeleton loading were fixed by checking scoped targets and animating after loading.

## Build checks

`npm run build` succeeds and exports all seven routes. `npm run lint` passes. Four `node --test lib/demo.test.mjs` checks pass: address validation, CSV escaping/formula protection, offline data, Windows process fixtures. TypeScript verification is included in the production build.

## Backend integration checks (0.2.0)

- Production static export served by Go at 127.0.0.1:8080; health API returns source=demo.
- Realtime selection displays SSE status, packet metadata and metric values update.
- Network filters narrow the sample to inbound DNS packets. Details show source/destination, flags, bytes and process. Local pause also freezes the chart.
- Memory sorting puts postgres (1.2 GB) ahead of nginx (512 MB); numeric helper tested separately.
- Disks expose capacity, free space, rates, IOPS, busy time, latency and temperature. Production GPU exposes utilization/VRAM/temperature/power/fan; Windows node shows no GPU and NTFS C:/D: volumes.
- Full log history contains more than 100 records. “К первым записям” opens page 101–127 and retains the first record at 16:35:26. Search and ERROR filters include old records.
- Backend restarted twice using the same data directory. First log ID ad982ff3f661a1ee305ac7c89a1e46c7 and timestamp persisted; new entries appended.
- Dark/light themes and realtime preference survive reload. OS icons render in monochrome in both themes.
- Mobile network view at 390×844 uses stacked layout with 375 px document scroll width (no horizontal document overflow). Temporary viewport override reset.
- No browser errors observed. The packet drawer no longer emits warnings for absent GSAP targets; historical warnings from the earlier build remain in browser log history.
- npm lint/typecheck/build pass; six Node tests pass. Eight Go tests pass and go vet passes. The archive-boundary regression and host guard were observed failing before their fixes.
- go test -race could not run because this Windows Go environment has CGO disabled. No race-check success is claimed.

## Current scope limitations

Go API, persistent server registry, JSONL log archive and SSE work. System data and packet metadata are explicitly synthetic; OS collection, GPU detection, packet capture and production notifications await the later Go agent/backend stages. The current backend only binds to loopback and rejects non-loopback Host headers. JSONL search scans the archive sequentially; large deployments will need indexed storage. Verification screenshots are ignored by Git.


## Authentication and hardware UI checks (0.3.0)

- Login verified in the browser using a disposable owner on a separate Go instance at 8081; the real data directory was not used for test credentials.
- Successful login opens the dashboard, displays the owner name, and logout returns to login with a local return path.
- GPU/VRAM columns display active python3/node/docker readings. GPU sorting places python3 first; configuration shows NVIDIA RTX 4090 and 24 GB VRAM.
- Add-server drawer has read-only automatic OS information, no OS selector or simulated connection check. A test node saved in the separate QA instance displays “Ожидаем агента” with no online metrics.
- Login checked in both monochrome themes. At 390×844, document client/scroll width both equal 375 px; viewport override reset. Screenshots: artifacts/login-dark.png, login-light.png, login-mobile.png.
- No browser console errors observed on the final login page.
- npm lint/typecheck/build pass; nine Node tests pass; 16 Go tests pass and go vet passes. Authentication tests cover owner persistence, closed signup, password limits, cookie flags, logout/CSRF, session expiration, rate limits and private monitoring routes. GPU/automatic-OS regressions covered.
- Go race verification remains unavailable: CGO is disabled. No race-check success is claimed.
- Final backend serves the updated static export at 127.0.0.1:8080. Authentication is local; remote HTTPS deployment and real agents are pending. Restarting the backend invalidates existing sessions.
