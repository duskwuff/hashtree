hashtree
========

A tiny little utility to generate hashes of all files in a directory tree.


Usage
-----

    hashtree [options] <path...>

Options:

* `-fmt <string>`

    Selects an output format. Options are:

    * `hex` (default)

        Hash (as lowercase hexadecimal), two spaces, file path. This is the
        format used by standard utilities like `sha256sum`.

    * `base64`

        Same as `hex`, but with Base64 output (standard form, with padding)
        instead of hex.

    * `json-hex` (or simply `json`)

        One JSON object on each line, with keys `path` (file path) and `hash`
        (lowercase hex hash).

    * `json-base64`

        Same as `json-hex`, but with Base64.

* `-hash <string>`

    Selects the hash to use. Supported hashes are currently:

    * `sha256` (default)
    * `sha224`
    * `sha512`
    * `sha1` (weak - avoid)
    * `md5` (weak - avoid)
    * `crc32` (not a cryptographic hash - uses IEEE polynomial)

* `-jobs <int>`

    Selects the number of jobs to run in parallel. By default, one job is used
    per CPU in the system.

    Note that, with more than one job running, the output order will be
    unpredictable. Consider piping output to a utility like `sort` if
    consistency is needed.
