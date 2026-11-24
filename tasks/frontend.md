# Updog Frontend Implementation Tasks

Bite-sized task list for building a sleek, responsive dashboard to manage monitors and view check metrics via the REST API.

---

# Milestone 1: Project Setup & UI Shell

## [ ] Task 01: Initialize Web Project Shell
- **Target File:** `web/index.html`
- [ ] Create HTML5 boiler shell with Google Fonts (`Inter`) and meta viewport tags
- [ ] Set up layout structure: Header, Stats Overview, Monitors Grid, Add Monitor Modal
- **DoD:** `web/index.html` loads cleanly in browser with proper semantic outline

## [ ] Task 02: Design System & Modern CSS Styling
- **Target File:** `web/style.css`
- [ ] Define CSS color tokens (dark mode theme, status colors: emerald for UP, rose for DOWN)
- [ ] Add glassmorphic card styles, button states, badge indicators, and grid responsive rules
- [ ] Add smooth micro-animations for status pulses and modal transitions
- **DoD:** Dashboard layout renders dark theme styled elements without external UI frameworks

---

# Milestone 2: API Integration & Data Binding

## [ ] Task 03: Implement API Client Module
- **Target File:** `web/js/api.js`
- [ ] Create `fetchMonitors()` using `fetch('/api/v1/monitors')`
- [ ] Create `createMonitor(data)` with JSON payload POST request
- [ ] Create `fetchMonitorStats(id)` for fetching history check logs
- **DoD:** `api.js` functions handle HTTP responses, parsing JSON, and handling errors gracefully

## [ ] Task 04: Monitor List & Card Rendering
- **Target File:** `web/js/components.js`
- [ ] Write `renderMonitorCard(monitor)` generating DOM element with status badge and response time
- [ ] Write `renderEmptyState()` for when no monitors exist
- **DoD:** Monitor cards dynamically render onto grid container when API returns data

---

# Milestone 3: Interactive Features & Real-Time Monitoring

## [ ] Task 05: Add Target Form & Validation
- **Target File:** `web/js/app.js`
- [ ] Attach submit event listener to Add Monitor form modal
- [ ] Validate URL format and ensure interval >= 10 seconds before sending payload
- [ ] Refresh monitor grid upon successful creation (HTTP 201)
- **DoD:** Submitting valid form creates a new monitor and updates dashboard list immediately

## [ ] Task 06: Detailed Stats & Response History View
- **Target File:** `web/js/modal.js`
- [ ] Create click listener on monitor card to open historical stats modal
- [ ] Display aggregate stats (uptime percentage, avg latency) and timeline of recent check results
- **DoD:** Clicking card displays pop-up modal populated with check history data from `/api/v1/monitors/{id}/stats`

## [ ] Task 07: Automatic Dashboard Live Polling
- **Target File:** `web/js/app.js`
- [ ] Implement `setInterval` timer (every 10s) to re-fetch monitors and update status badges smoothly
- [ ] Add manual refresh button with visual spinning state
- **DoD:** Dashboard updates monitor status automatically in background without full page reload
