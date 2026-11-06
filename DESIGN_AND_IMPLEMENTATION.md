# Ping Enhancer — System Design & Implementation Document

This document provides the complete technical specification for **Ping Enhancer**, covering:
1. **Product & Routing Architecture** (`Direct` vs. `IR Relay` nodes, per-game split routing, HWID + Account hybrid quota model)
2. **PostgreSQL & Go `ent` ORM Schema** (mapped directly to the `XApi` database schema)
3. **WGDashboard Integration & Traffic Accounting Worker**
4. **Go Fiber REST API Specification**
5. **Flutter Android Client Implementation Guide** (1:1 mapping with the React Material 3 UI/UX prototype)

---

## 1. System Architecture Overview

```
┌──────────────────────────────────────────────────────────────────────────┐
│                     Flutter Android App ("Ping Enhancer")                │
│  • Material 3 Dark Theme (Home, ICMP, Plans, Profile, Login)             │
│  • Per-Game Split Routing (Android VpnService.addAllowedApplication)     │
│  • Native ICMP Engine (dart_ping: Quick Ping & 10x Deep Probe Suite)     │
│  • Custom DNS Resolver Injection (Auto, 1.1.1.1, 8.8.8.8, Electro, etc.) │
└────────────────────────────────────┬─────────────────────────────────────┘
                                     │ HTTPS (Bearer Token + X-Device-HWID)
                                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                     Go Fiber API Backend (XApi) + Ent ORM                │
│  • Auth Service (Email/Password + Session Token + HWID Binding)          │
│  • Quota & Subscription Engine (Pulse 25GB, Vortex 60GB, Apex 150GB)     │
│  • Ad Reward Verifier (+200 MB top-up on devices/subscriptions)          │
│  • WGDashboard Orchestrator & 30s Traffic Sync Background Worker         │
└──────────────┬───────────────────────────────────────────┬───────────────┘
               │ SQL (ent/sql)                             │ REST API (wg-dashboard-apikey)
               ▼                                           ▼
┌──────────────────────────────┐            ┌──────────────────────────────┐
│     PostgreSQL Database      │            │   Server Fleet (WGDashboard) │
│  • games                     │            │  • Direct Foreign VPS Nodes  │
│  • users                     │            │    (TR, DE, NL, AE)          │
│  • devices (HWID)            │            │  • IR Relay Entry Nodes      │
│  • subscriptions             │            │    (Iranian VPS ──► Foreign) │
│  • configs                   │            └──────────────────────────────┘
└──────────────────────────────┘
```

### Core Product & UX Rules
1. **Terminology**: Never mention `"VPN"`, `"WireGuard"`, or `"Split Tunneling"` anywhere in the user-facing UI. Use **"Ping Enhancer"**, **"Selected Games"**, **"IR Relay"**, and **"Direct"**.
2. **Dual-Mode Server Topology**:
   - **Direct (`direct`)**: The client connects directly to the foreign game VPS (`TR`, `DE`, `NL`, `AE`) where WireGuard + WGDashboard run.
   - **IR Relay (`relay`)**: The client connects to an Iranian relay server IP/port (clearing ISP international throttling), which forwards encrypted packets over a persistent tunnel/backbone to the foreign VPS.
3. **Hybrid HWID + Account Quota System**:
   - **No Account Required to Test**: Every device is identified by a hardware fingerprint (`hwid`) and receives **500 MB daily free traffic** bound to its `devices` record in PostgreSQL so clearing app data or reinstalling cannot abuse the free quota.
   - **Rewarded Ad Top-Up (`+Ad`)**: Tapping `+Ad` in the top app bar watches a short rewarded ad and adds **+200 MB** (`+0.2 GB`) to the user's available traffic.
   - **Monthly Subscriptions (`Pulse`, `Vortex`, `Apex`)**: All plans have **identical routing quality** and **30-day duration**, differentiated strictly by monthly traffic quota (`25 GB`, `60 GB`, `150 GB`).

---

## 2. PostgreSQL & Go `ent` Schema Specification (`XApi`)

### 2.1 Current `atlas schema inspect` Baseline

Your current `XApi` schema defines 5 core tables:
- `games`: `id`, `name`, `process_name`
- `users`: `id`, `name`, `email` (unique), `password`, `session_token`, `created_at`, `updated_at`
- `devices`: `id`, `hwid` (unique), `bytes_total`, `bytes_used`, `user_id` (unique 1:1 FK to `users.id`)
- `subscriptions`: `id`, `is_active`, `tier`, `price`, `bytes_total`, `bytes_used`, `start_date`, `end_date`, `purchase_date`, `user_id` (FK to `users.id`)
- `configs`: `id`, `location`, `value`, `name`, `device_id` (FK to `devices.id`), `subscription_id` (nullable FK to `subscriptions.id`)

### 2.3 Complete Go `ent/schema` Definitions

#### `ent/schema/game.go`
```go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Game struct {
	ent.Schema
}

func (Game) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),         // Game label, e.g., "PUBG Mobile"
		field.String("process_name").NotEmpty(), // Android package ID, e.g., "com.tencent.ig"
	}
}
```

#### `ent/schema/user.go`
```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Default(""),
		field.String("email").Unique().NotEmpty(),
		field.String("password").Sensitive().NotEmpty(),
		field.String("session_token").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (User) Edges() []ent.Field {
	return nil
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("device", Device.Type).Unique(),
		edge.To("subscriptions", Subscription.Type),
	}
}
```

#### `ent/schema/device.go`
```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Device struct {
	ent.Schema
}

func (Device) Fields() []ent.Field {
	return []ent.Field{
		field.String("hwid").Unique().NotEmpty(),
		field.Int64("bytes_total").Default(524288000), // 500 MB daily free traffic
		field.Int64("bytes_used").Default(0),
		field.Time("daily_reset_at").Default(func() time.Time {
			return time.Now().Add(24 * time.Hour)
		}),
		field.Int("user_id").Optional().Nillable(),
	}
}

func (Device) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("device").
			Field("user_id").
			Unique(),
		edge.To("configs", Config.Type),
	}
}
```

#### `ent/schema/subscription.go`
```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Subscription struct {
	ent.Schema
}

func (Subscription) Fields() []ent.Field {
	return []ent.Field{
		field.Bool("is_active").Default(true),
		field.String("tier").NotEmpty(), // "Pulse", "Vortex", "Apex"
		field.Int64("price").Positive(), // Cents (e.g., 249, 449, 799) or local currency unit
		field.Int64("bytes_total").Default(0),
		field.Int64("bytes_used").Default(0),
		field.Time("start_date").Default(time.Now),
		field.Time("end_date"),
		field.Time("purchase_date").Default(time.Now),
		field.Int("user_id"),
	}
}

func (Subscription) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("subscriptions").
			Field("user_id").
			Unique().
			Required(),
		edge.To("configs", Config.Type),
	}
}
```

#### `ent/schema/config.go`
```go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Config struct {
	ent.Schema
}

func (Config) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),         // e.g., "Istanbul 01", "Frankfurt 01"
		field.String("location").NotEmpty(),     // e.g., "TR", "DE", "NL", "AE"
		field.Enum("route_type").
			Values("relay", "direct").
			Default("relay"),                    // "relay" ("IR Relay") | "direct" ("Direct")
		field.String("ping_address").NotEmpty(), // IP/Host used for ICMP Ping & Deep Test
		field.String("value").NotEmpty(),        // Full WireGuard INI configuration string
		field.String("wg_public_key").Optional(),
		field.Int64("wg_last_total_bytes").Default(0),
		field.Int("device_id"),
		field.Int("subscription_id").Optional().Nillable(),
	}
}

func (Config) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("device", Device.Type).
			Ref("configs").
			Field("device_id").
			Unique().
			Required(),
		edge.From("subscription", Subscription.Type).
			Ref("configs").
			Field("subscription_id").
			Unique(),
	}
}
```

### 2.4 Resulting PostgreSQL DDL (After Running `atlas schema inspect`)
```sql
-- Create "games" table
CREATE TABLE "games" (
  "id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY,
  "name" character varying NOT NULL,
  "process_name" character varying NOT NULL,
  PRIMARY KEY ("id")
);

-- Create "users" table
CREATE TABLE "users" (
  "id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY,
  "name" character varying NOT NULL DEFAULT '',
  "email" character varying NOT NULL,
  "password" character varying NOT NULL,
  "session_token" character varying NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "users_email_key" ON "users" ("email");

-- Create "devices" table
CREATE TABLE "devices" (
  "id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY,
  "hwid" character varying NOT NULL,
  "bytes_total" bigint NOT NULL DEFAULT 524288000,
  "bytes_used" bigint NOT NULL DEFAULT 0,
  "daily_reset_at" timestamptz NOT NULL,
  "user_id" bigint NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "devices_users_device" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
CREATE UNIQUE INDEX "devices_hwid_key" ON "devices" ("hwid");
CREATE UNIQUE INDEX "devices_user_id_key" ON "devices" ("user_id");

-- Create "subscriptions" table
CREATE TABLE "subscriptions" (
  "id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY,
  "is_active" boolean NOT NULL DEFAULT true,
  "tier" character varying NOT NULL,
  "price" bigint NOT NULL,
  "bytes_total" bigint NOT NULL DEFAULT 0,
  "bytes_used" bigint NOT NULL DEFAULT 0,
  "start_date" timestamptz NOT NULL,
  "end_date" timestamptz NOT NULL,
  "purchase_date" timestamptz NOT NULL,
  "user_id" bigint NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "subscriptions_users_subscription" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

-- Create "configs" table
CREATE TABLE "configs" (
  "id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY,
  "name" character varying NOT NULL,
  "location" character varying NOT NULL,
  "route_type" character varying NOT NULL DEFAULT 'relay',
  "ping_address" character varying NOT NULL,
  "value" character varying NOT NULL,
  "wg_public_key" character varying NULL,
  "wg_last_total_bytes" bigint NOT NULL DEFAULT 0,
  "device_id" bigint NOT NULL,
  "subscription_id" bigint NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "configs_devices_configs" FOREIGN KEY ("device_id") REFERENCES "devices" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "configs_subscriptions_configs" FOREIGN KEY ("subscription_id") REFERENCES "subscriptions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
```

---

## 3. WGDashboard Integration & Traffic Sync Architecture

### 3.1 How Direct and IR Relay Servers Work with WGDashboard
Each foreign game server (e.g. `Istanbul 01`, `Frankfurt 01`, `Amsterdam 01`, `Dubai 01`) runs WireGuard (`wg0`) and **WGDashboard**.
- **Direct Config (`route_type = 'direct'`)**:
  - `ping_address`: Foreign VPS public IP (e.g., `91.107.x.x`)
  - WireGuard `Endpoint` inside `configs.value`: `91.107.x.x:51820`
- **IR Relay Config (`route_type = 'relay'`)**:
  - `ping_address`: Iranian Relay VPS public IP (e.g., `185.141.x.x`)
  - WireGuard `Endpoint` inside `configs.value`: `185.141.x.x:51820`
  - The Iranian Relay VPS forwards UDP traffic (`iptables`/`nftables` DNAT or `socat`/`Gost` tunnel) to `91.107.x.x:51820`.
  - Both routes can be managed through the **same** WGDashboard instance on the foreign VPS.

### 3.2 WGDashboard HTTP API Calls from Go Fiber
Authenticate all requests to WGDashboard using the `wg-dashboard-apikey: <API_KEY>` header:

1. **Create Peer (`POST /api/addPeers/{configName}`)**:
   - Called when provisioning `configs` for a `device_id`.
   - Generates a client WireGuard keypair, registers the public key on WGDashboard, and stores the full client INI string in `configs.value` and the public key in `configs.wg_public_key`.
2. **Restrict Peer (`POST /api/restrictPeers/{configName}`)**:
   - Called when both the user's active subscription quota and device free quota reach `0`, or when `end_date` passes.
3. **Allow Peer (`POST /api/allowAccessPeers/{configName}`)**:
   - Called when the user purchases/renews a subscription, watches a rewarded ad (`+Ad`), or gets their daily 500 MB reset.
4. **Poll Traffic (`GET /api/getWireguardConfigurationInfo?configurationName={configName}`)**:
   - Polled every **30 seconds** by the Go background worker.

### 3.3 Go Background Traffic Accounting Worker (30-Second Ticker)
```go
// Pseudocode for internal/worker/traffic_sync.go
func SyncTrafficTick(ctx context.Context, client *ent.Client, nodes []WGDNode) {
    for _, node := range nodes {
        peers := wgClient.GetPeers(node.URL, node.APIKey, node.Interface)
        for _, p := range peers {
            cfg, err := client.Config.Query().
                Where(config.WgPublicKeyEQ(p.PublicKey)).
                WithDevice().
                WithSubscription().
                Only(ctx)
            if err != nil {
                continue
            }

            currentBytes := p.TotalReceiveBytes + p.TotalSentBytes + p.CumuReceiveBytes + p.CumuSentBytes
            delta := currentBytes - cfg.WgLastTotalBytes
            if delta <= 0 {
                continue
            }

            // Update last synced counter
            _ = client.Config.UpdateOneID(cfg.ID).SetWgLastTotalBytes(currentBytes).Exec(ctx)

            // Deduct from active subscription first; fallback to device HWID quota
            sub := cfg.Edges.Subscription
            dev := cfg.Edges.Device
            now := time.Now()

            if sub != nil && sub.IsActive && now.Before(sub.EndDate) && sub.BytesUsed < sub.BytesTotal {
                newUsed := sub.BytesUsed + delta
                stillActive := newUsed < sub.BytesTotal
                _ = client.Subscription.UpdateOneID(sub.ID).
                    SetBytesUsed(newUsed).
                    SetIsActive(stillActive).
                    Exec(ctx)

                if !stillActive && dev.BytesUsed >= dev.BytesTotal {
                    wgClient.RestrictPeer(node, p.PublicKey)
                }
            } else if dev != nil {
                newDevUsed := dev.BytesUsed + delta
                _ = client.Device.UpdateOneID(dev.ID).
                    SetBytesUsed(newDevUsed).
                    Exec(ctx)

                if newDevUsed >= dev.BytesTotal {
                    wgClient.RestrictPeer(node, p.PublicKey)
                }
            }
        }
    }
}
```

---

## 4. Go Fiber REST API Specification

### Global Headers
- `X-Device-HWID`: Required on every request (e.g., `ANDR-9F4A-882C`).
- `Authorization`: Optional `Bearer <session_token>` (matches `users.session_token`).

### 4.1 `GET /api/v1/bootstrap`
Initializes the app state on startup.
- Upserts the `devices` row by `hwid`.
- Resets `devices.bytes_used = 0` if `NOW() > devices.daily_reset_at`.
- Returns:
  - `device`: `{ "hwid", "bytes_total", "bytes_used", "daily_reset_at" }`
  - `user`: `{ "id", "email", "is_logged_in" }` (if `session_token` is valid or linked to `devices.user_id`)
  - `active_subscription`: `{ "id", "tier", "days_remaining", "bytes_total", "bytes_used", "remaining_gb" }` (or `null` if on free tier)
  - `configs`: Array of `{ "id", "name", "location", "route_type", "ping_address" }`
  - `games`: Array of `{ "id", "name", "process_name" }`

### 4.2 `POST /api/v1/connect`
Validates remaining traffic quota and returns the WireGuard config for the selected server.
- **Request**: `{ "config_id": 101 }`
- **Response**:
  ```json
  {
    "config_id": 101,
    "name": "Istanbul 01",
    "location": "TR",
    "route_type": "relay",
    "value": "[Interface]\nPrivateKey = ...\nAddress = 10.66.66.14/32\nDNS = 1.1.1.1\n\n[Peer]\nPublicKey = ...\nEndpoint = 185.141.x.x:51820\nAllowedIPs = 0.0.0.0/0"
  }
  ```

### 4.3 `POST /api/v1/traffic/ad-reward`
Triggered by the **`+Ad`** button in the top AppBar after a rewarded ad finishes.
- Adds `209715200` bytes (`+200 MB` / `+0.2 GB`) to the active subscription's `bytes_total` (if active) or `devices.bytes_total`.
- Unrestricts peers in WGDashboard if previously restricted.

### 4.4 `POST /api/v1/auth/register` & `POST /api/v1/auth/login`
- **Register**: Creates `users` row (`email`, `bcrypt(password)`, auto-derived `name`), generates `session_token`, and links `devices.user_id = user.ID`.
- **Login**: Verifies credentials, rotates `users.session_token`, updates `devices.user_id = user.ID` (clearing `user_id` on any previous device because `devices_user_id_key` is unique), and returns the user's active subscription and purchase history.
- **Logout (`POST /api/v1/auth/logout`)**: Clears `users.session_token = NULL` and unlinks `devices.user_id = NULL` so the device falls back to its hardware-bound 500 MB daily quota.

### 4.5 `POST /api/v1/subscriptions/purchase` & `GET /api/v1/subscriptions/history`
- **3 Monthly Plans Catalog**:
  - **Pulse**: `tier = "Pulse"`, `price = 249` ($2.49), `bytes_total = 26843545600` (25 GB), 30 Days
  - **Vortex**: `tier = "Vortex"`, `price = 449` ($4.49), `bytes_total = 64424509440` (60 GB), 30 Days
  - **Apex**: `tier = "Apex"`, `price = 799` ($7.99), `bytes_total = 161061273600` (150 GB), 30 Days
- **`POST /api/v1/subscriptions/purchase`**:
  - Inserts a new row into `subscriptions` (`is_active = true`, `start_date = now`, `end_date = now + 30d`, `purchase_date = now`) and links the device's `configs.subscription_id`.
- **`GET /api/v1/subscriptions/history`**:
  - Queries `subscriptions` where `user_id = currentUser.ID` ordered by `purchase_date DESC` to populate the **Purchase History** card in the **Profile** tab.

---

## 5. Flutter Client Implementation Guide (1:1 Match with UI Prototype)

### 5.1 Design Tokens (Material 3 Dark Theme)
- **App Background**: `#0B0E15`
- **Surface Card**: `#121620` with `1px` border `#1E293B` (`slate-800/80`) and `24px` (`rounded-3xl`) radius
- **Inner Pill / Control Surface**: `#0B0E15` or `#191E2B`
- **Primary Accent (Connected / IR Relay / Active Selection)**: Emerald `#34D399` (`emerald-400`)
- **Secondary Accent (Direct Route Label)**: Sky Blue `#38BDF8` (`sky-400`)
- **Warning / Max Ping Accent**: Amber `#FCD34D` (`amber-300`)
- **Timeout / Packet Loss Accent**: Rose `#FB7185` (`rose-400`)

### 5.2 Screen-by-Screen Specification

#### 1. Top AppBar
- **Left**: `"Ping Enhancer"` title.
- **Right Cluster**:
  - **Plan & Traffic Status Pill**: Shows `{tier} · {days}d · {remaining_gb} GB` (e.g., `Vortex · 22d · 42.5 GB`), or `Free · 1d · 0.5 GB` if no active subscription. No clock icon.
  - **Separate `+Ad` Pill Button**: Placed to the right of the plan status pill. Triggers a rewarded ad and increments traffic by `+0.2 GB`.

#### 2. Home Tab
- **Connection Card**:
  - **Top**: Centered circular `112x112` tactile Connect/Disconnect button with `Power` icon, status label (`Connect` / `Connecting` / `Connected`), and `HH:MM:SS` session timer.
  - **Bottom Row (below the Connect button)**: Left side displays **only the server name** (`Istanbul 01`, no `TR` or `DE` prefix); right side displays the server's ping (`24 ms`).
- **Selected Games Trigger Card & Modal Bottom Sheet**:
  - **Modal Header**: Single horizontal row with `"Selected Games"` on the left (no `3/7` counter, no subtitle) vertically centered with `"All"`, `"Clear"`, and `"X"` buttons on the right.
  - **Search Bar**: Filters `games` in real time by matching either `game.name` (label) or `game.process_name` (Android package ID, e.g., `com.tencent.ig`).
  - **Per-Game Split Routing on Android**:
    When starting the tunnel, pass the selected `process_name` values into Android's `VpnService.Builder`:
    ```kotlin
    for (packageName in selectedProcessNames) {
        try {
            vpnBuilder.addAllowedApplication(packageName)
        } catch (_: PackageManager.NameNotFoundException) {}
    }
    ```
- **Server List Card**:
  - **Category Filter**: `All | Relay | Direct` segmented control.
  - **Toolbar Row**:
    - **Search Input**: Filters servers by `name` (`Istanbul 01`) or `location` (`TR`, `DE`, `NL`, `AE`).
    - **`Sort` Button**: Cycles `Sort` (Default) $\rightarrow$ `Ping ↑` (Ascending) $\rightarrow$ `Ping ↓` (Descending).
    - **`Ping` Button**: Pings all `configs.ping_address` hosts (`count: 1`) via `dart_ping` and updates each server's `ms`.
  - **Server Row**:
    - Left: `[TR]` 2-letter location box + `Istanbul 01`.
    - Right: `24 ms` on top, colored `IR Relay` (emerald) or `Direct` (sky-blue) underneath, and the **green tick circle** at the far right.

#### 3. ICMP Tab
- **Unified All-Servers Deep Test Screen**:
  - **Top Summary Card**: Title `ICMP Server Test`, subtitle `Gathering & testing server routes`, comparison strip (`IR Relay Avg` vs `Direct Avg`), and a single primary button: **`Start Deep ICMP Test (All Servers)`**.
  - Pressing the button runs 10 ICMP echo probes (`Ping(cfg.pingAddress, count: 10)`) across all servers and updates their attached stats directly on the ICMP screen.
  - **Filter & Multi-Metric Sort Controls**:
    - Route filter: `All | Relay | Direct`.
    - Sort metric pills: **`Avg Ping`**, **`Min Ping`**, **`Loss`**, paired with an **`↑ Asc` / `↓ Desc`** toggle button.
  - **Server Result Cards**:
    - Top row: `[TR]` badge + `Istanbul 01` + colored `IR Relay` / `Direct` on the left; **`⚡` (Single-Server Ping Log)** button + rightmost **green tick circle** on the right.
    - Bottom row: Attached 4-column stats grid (**`Min`**, **`Avg`**, **`Max`**, **`Loss %`**).
- **Single-Server Attempted Pings Drilldown (`⚡` button)**:
  - Top card: `← All Servers` back button, `Retest` button, selection tick circle, server title, and 4-column `Min / Avg / Max / Loss` boxes.
  - Bottom card (`Attempted ICMP Pings`): Lists all 10 probes (`#1   64B · TTL 58` on the left, `23 ms` in emerald or `Timeout` in rose on the right, with no `"Reply"` text badge).

#### 4. Plans Tab
- Displays the 3 monthly plans (**Pulse 25 GB**, **Vortex 60 GB**, **Apex 150 GB**) with explicit **`Purchase` / `Renew`** buttons.
- Selecting a plan opens the inline **Order Summary & Payment Checkout** card with payment method selection (**Direct Bank Card** or **USDT TRC-20**), traffic/duration summary, and final **Purchase** confirmation.

#### 5. Profile Tab
- Ordered by importance:
  1. **User Account & Active Plan Card**: Shows `users.email`, active plan badge, `devices.hwid`, and `Sign Out` / `Sign In` button.
  2. **Purchase History Card**: Displays formatted `subscriptions` records (`INV-2026-xxx`, `Vortex · 60 GB (30 Days)`, date, price, `Active`/`Expired`).
  3. **Settings Card**:
     - **Enable Notifications** toggle switch.
     - **DNS Server** Material 3 expandable radio list (`Automatic`, `Cloudflare 1.1.1.1`, `Google 8.8.8.8`, `Quad9 9.9.9.9`, `Electro 78.157.42.100`, `Shecan 178.22.122.100`). Before activating the tunnel, replace the `DNS = ...` line in `configs.value` with the selected DNS IP.

#### 6. Login Tab
- Starts directly with the **`Sign In` / `Create Account`** segmented switcher (no profile avatar or welcome header), followed by the Email/Password form card and the bottom **Device HWID** card (`"No account needed to test. Every device receives 500 MB daily free traffic automatically bound to its hardware ID."`, with no "Return to Home" button).
