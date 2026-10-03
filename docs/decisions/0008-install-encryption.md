# 0008: Per-mod install encryption

- Status: Accepted
- Date: 2026-10-03

## Decision

Each mod has its own encryption toggle in the shared install preview.
Clear mods start with the toggle off. Encrypted mods stay encrypted.
Classic and incomplete bundles cannot request encryption.

Go encrypts staged bundles before it copies any files into the destination library.
It reuses the existing IoStore rebuild and companion cleanup behavior.
It uses one write-timeout worker for the install.
It processes each bundle separately so mixed source encryption states remain valid.
Any rebuild failure stops the install. The source files stay intact.

This decision replaces the install-time exclusion in decision 0005.
The game key stays in Go. Install-time decryption stays out of scope.
