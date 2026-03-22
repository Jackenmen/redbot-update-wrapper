from __future__ import annotations

import functools
import os
import re
import shutil
import subprocess
import sys
import sysconfig
from typing import Any, Dict, Optional, Tuple

from hatchling.builders.hooks.plugin.interface import BuildHookInterface

_GO_MOD_TOOLCHAIN_RE = re.compile(r"^toolchain go(\d+(\.\d+)*)", re.MULTILINE)
_GO_MOD_VERSION_RE = re.compile(r"^go (\d+(\.\d+)*)", re.MULTILINE)
_GO_VERSION_CMD_RE = re.compile(r"^go version go([^ ]+) ")
_GO_VERSION_RE = re.compile(r"^((?P<major>\d+)\.(?P<minor>\d+)(?:\.(?P<rev>\d+))?)(?P<extra>.*)$")


class GoVersion:
    def __init__(self, version: str, /) -> None:
        match = _GO_VERSION_RE.match(version)
        if match is None:
            raise ValueError("unexpected version number")
        self.major = int(match["major"])
        self.minor = int(match["minor"])
        self.rev = int(match["rev"])
        self.extra = match["extra"]

    def __str__(self) -> str:
        return f"{self.major}.{self.minor}.{self.rev}{self.extra}"

    @property
    def _key(self):
        return (self.major, self.minor, self.rev, self.extra)

    def __eq__(self, other: Any) -> bool:
        if isinstance(other, self.__class__):
            return self._key == other._key
        return NotImplemented

    def __ne__(self, other: Any) -> bool:
        if isinstance(other, self.__class__):
            return self._key != other._key
        return NotImplemented

    def __lt__(self, other: Any) -> bool:
        if isinstance(other, self.__class__):
            return self._key < other._key
        return NotImplemented

    def __le__(self, other: Any) -> bool:
        if isinstance(other, self.__class__):
            return self._key <= other._key
        return NotImplemented

    def __gt__(self, other: Any) -> bool:
        if isinstance(other, self.__class__):
            return self._key > other._key
        return NotImplemented

    def __ge__(self, other: Any) -> bool:
        if isinstance(other, self.__class__):
            return self._key >= other._key
        return NotImplemented


_TOOLCHAIN_SUPPORTED_SINCE = GoVersion("1.21.0")


class GoMod:
    def __init__(self, contents: str) -> None:
        self._contents = contents

        match = _GO_MOD_TOOLCHAIN_RE.search(contents)
        if match is None:
            raise RuntimeError("could not find toolchain directive in go.mod")
        self.toolchain_version = GoVersion(match[1])
        match = _GO_MOD_VERSION_RE.search(contents)
        if match is None:
            raise RuntimeError("could not find go directive in go.mod")
        self.minimum_go_version = GoVersion(match[1])

    @classmethod
    def from_file(cls, path: str, /) -> GoMod:
        with open(path, encoding="utf-8") as fp:
            return cls(fp.read())


def _get_go_mod() -> str:
    return GoMod.from_file("go.mod")


def _get_system_go() -> Union[Tuple[None, None], Tuple[str, GoVersion]]:
    bin_path = shutil.which("go")
    if bin_path is None:
        return None
    try:
        version_output = subprocess.check_output((bin_path, "version"), encoding="utf-8")
    except subprocess.CalledProcessError:
        return None
    match = _GO_VERSION_CMD_RE.match(version_output)
    if match is None:
        return None
    try:
        go_version = GoVersion(match.group(1))
    except ValueError:
        return None
    return bin_path, go_version


def _get_go_bin_args(*, prefer_system: bool = True) -> Tuple[str, ...]:
    if prefer_system:
        system_go_bin, system_go_version = _get_system_go()
        if system_go_version is not None and system_go_version >= _TOOLCHAIN_SUPPORTED_SINCE:
            return (system_go_bin,)

    return (sys.executable, "-m", "go")


class GoTarget:
    def __init__(self, goos: str, goarch: str) -> None:
        self.os = goos
        self.arch = goarch
        if not goos:
            raise ValueError("goos argument cannot be empty")
        if not goarch:
            raise ValueError("goarch argument cannot be empty")

    @classmethod
    def from_env(cls) -> Optional[GoTarget]:
        goos = os.getenv("GOOS")
        goarch = os.getenv("GOARCH")
        if (not goos and goarch) or (goos and not goarch):
            raise RuntimeError(
                "You can either specify both GOOS and GOARCH or specify neither."
                " Partially defined target is not supported."
            )
        if goos:
            return cls(goos, goarch)
        return None


class CustomBuildHook(BuildHookInterface):
    @property
    def _force_use_go_bin(self) -> bool:
        return bool(int(os.getenv("FORCE_USE_GO_BIN_PIP_PACKAGE", "0")))

    @functools.cached_property
    def _go_target(self) -> Optional[GoTarget]:
        return GoTarget.from_env()

    @functools.cached_property
    def _platform_tag(self) -> str:
        platform_tag = os.getenv("PLATFORM_TAG")
        if platform_tag is not None:
            if self._go_target is None:
                raise RuntimeError(
                    "Custom PLATFORM_TAG can only be set when GOOS and GOARCH are specified."
                )
        elif self._go_target is not None:
            raise RuntimeError(
                "GOOS and GOARCH can only be specified, if custom PLATFORM_TAG is also set."
            )
        else:
            platform_tag = "py3-none-" + (
                sysconfig.get_platform().replace(".", "_").replace("-", "_").replace(" ", "_")
            )
        return platform_tag

    @property
    def _binary_name(self) -> str:
        is_windows = (
            self._go_target.os == "windows" if self._go_target else sys.platform == "win32"
        )
        return "redbot-update.exe" if is_windows else "redbot-update"

    @property
    def _binary_dir(self) -> str:
        return os.path.join(self.directory, "go-output", self._platform_tag)

    @property
    def _binary_path(self) -> str:
        return os.path.join(self._binary_dir, self._binary_name)

    def dependencies(self) -> List[str]:
        if not self._force_use_go_bin:
            _, system_go_version = _get_system_go()
            # When Go toolchains (https://tip.golang.org/doc/toolchain) are supported,
            # the system Go shouldn't need to be newer than `go_mod.minimum_go_version`
            # as `go` will automatically download it.
            if system_go_version is not None and system_go_version >= _TOOLCHAIN_SUPPORTED_SINCE:
                return []

        go_mod = _get_go_mod()
        return [f"go-bin=={go_mod.toolchain_version}"]

    def initialize(self, version: str, build_data: Dict[str, Any]) -> None:
        if self.target_name == "sdist":
            return

        if version == "editable":
            binary_path = (
                "Scripts\\redbot-update.exe" if sys.platform == "win32" else "bin/redbot-update"
            )
            # this won't actually be displayed by the frontend but what can you do...
            self.app.display_warning(
                "redbot-update binary is not included in an editable wheel."
                f" You should manually add a symlink to it at <venv dir>/{binary_path}"
            )
            return

        build_data["pure_python"] = False
        build_data["tag"] = self._platform_tag

        env = os.environ.copy()
        # avoid use of CGO to ensure we get static binaries
        env["CGO_ENABLED"] = "0"

        go_bin_args = _get_go_bin_args(prefer_system=not self._force_use_go_bin)
        build_command = (
            *go_bin_args,
            "build",
            "-ldflags=-s -w",
            "-o",
            self._binary_path,
            "./go/cmd/redbot-update",
        )

        self.app.display_info("Building redbot-update binary")
        os.makedirs(self._binary_dir, exist_ok=True)
        subprocess.check_call(build_command, env=env)
        self.app.display_success("Built redbot-update binary")

        build_data["shared_scripts"][self._binary_path] = self._binary_name

    def finalize(self, version: str, build_data: Dict[str, Any], artifact_path: str) -> None:
        if self.target_name == "sdist" or version == "editable":
            return
        os.unlink(self._binary_path)
