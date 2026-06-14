---
title: "Introduction"
description: "What sinanews is and how it is put together."
weight: 10
---

Browse Sina News articles (新浪新闻)

sinanews is a single binary. It speaks to sinanews-cli over plain HTTPS,
shapes the responses into clean records, and gets out of your way. There is
nothing to sign up for and nothing to run alongside it.

## How it is built

- A **library package** (`sinanews-cli`) holds the HTTP client and the typed
  data models. It paces requests, sets an honest User-Agent, and retries the
  transient failures any public site throws under load.
- A **command tree** (`cli`) wraps the library in subcommands with shared
  output formats and flags.
- One **`cmd/sinanews`** entry point ties them together.

## Scope

sinanews is a read-only client over data sinanews-cli already serves
publicly. It reads that data and shapes it for you. That narrow scope keeps it a
single small binary with no database, no daemon, and no setup.

Next: [install it](/getting-started/installation/), then take the
[quick start](/getting-started/quick-start/).
