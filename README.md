# portal
magic-wormhole, but worse...

This is project for the IK2218 Protocols and Principles of the Internet

This project was completed by:

- Zino Kader
- Arvid Gotthard 
- Anton Sederlin


## The application

`portal` is a file transfer utility, inspired by [magic-wormhole](https://github.com/magic-wormhole/magic-wormhole).

The file transfer starts by invoking the command from the sender side:

```bash
# portal send <file1> <file2>

```
The application will the output a temporary passphrase on the format,`1-inertia-elliptical-celestial`. The sender will communicate this passphrase to the receiver over some secure channel. The receiver would then issue the command:

```bash
# portal receive 1-intertia-elliptical-celestial

```

Then the two applications will connect to each other and transfer the file/files.

### Demo

![demo](./assets/demo.gif)

## Features

`portal` provides:

- End-to-end encryption, using [PAKE2](https://en.wikipedia.org/wiki/Password-authenticated_key_agreement) to negotiate a shared session-key.
- Direct transfer of files over websockets if sender and receiver are behind the same NAT.
- Fallback to TURN-server(rendezvous-relay) for file transfer if the sender and receiver are behind different NATs.
- Parallel gzip compression of files for faster transfer.

## Technical details

The connection between the sender and the server is negotiated using a intermediary server called `portal-rendezvous` server. The `portal-rendezvous` server is used to negotiate secure encrypted channel while never seeing the contents of files or the temporary passphrase.

The communication goes as follows

- `sender` application connects to `rendezvous-server`
- `rendezvous-server` allocates an id to the sender and sends over websocket to the `sender`
- `sender` outputs the passphrase to the terminal, hashes the passphrase and sends it to the `rendezvous-server`
- `receiver` hashes the passphrase (which has been communicated over some secure channel) and the sends it to the `rendezvous-server`
- When both the `sender` and the `receiver` has sent the hashed passphrase to the `rendezvous-server` the cryptographic exchange starts, during which the `rendezvous-server` relays messages from the `sender` to the `receiver` or vice versa.
- Once the cryptographic exchange is done, every message sent by the `sender` and `receiver` is encrypted, and the `rendezvous-server` cannot decrypt them.
- Now two things can happen: 
  - Either the `sender` and `receiver` are behind the same NAT, in which case the file transfer will be directly between the `sender` and `receiver`. In this case, the connection to `rendezvous-server` will be closed.
  - If they are not behind the same `NAT`, the transfer will fallback to go through the `rendezvous-server` who will continue to relay encrypted messages until the transfer is done.


## Software used

- Go standard libraries
- [gorilla/websocket](https://github.com/gorilla/websocket)
- [shollz/pake](https://github.com/schollz/pake)
- [atotto/clipboard](https://github.com/atotto/clipboard)
- [charmbracelet/bubbles](github.com/charmbracelet/bubbles)
- [charmbracelet/bubbletea](github.com/charmbracelet/bubbletea)
- [charmbracelet/lipgloss](github.com/charmbracelet/lipgloss)
- [klauspost/pgzip](github.com/klauspost/pgzip)
- [stretchr/testify](github.com/stretchr/testify)

