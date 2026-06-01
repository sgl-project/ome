# Agent Notes

- Keep the project on Go 1.24 for now. Starting with Go 1.25, the standard library's `crypto/tls` package enforces EMS for TLS 1.2 when running in FIPS 140-3 mode, and our LBAAS does not support TLS 1.3 yet.
- LBAAS is not ready yet; some teams still have issues with Go 1.25.
- Discussion thread: https://dyn.slack.com/archives/CDPAZCDTM/p1755295547701389
- LBAAS tracking ticket: https://jira-sd.mc1.oracleiaas.com/browse/LBFLAM-26129
