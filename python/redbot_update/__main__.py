import os
import sys
from typing import NoReturn

from . import find_redbot_update_bin

__all__ = ("find_redbot_update_bin",)


def main() -> NoReturn:
    binary_path = find_redbot_update_bin()
    if sys.platform == "win32":
        # Windows's exec is not capable of *actually* replacing the process so we can't use it.
        print(
            "Running redbot-update with `python -m redbot_update` command is unsupported on"
            f" Windows. You need to run the redbot-update binary directly:\n{binary_path}",
            file=sys.stderr,
        )
        raise SystemExit(1)

    os.execvp(binary_path, (binary_path, *sys.argv[1:]))


if __name__ == "__main__":
    main()
