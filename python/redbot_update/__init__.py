import os
import sysconfig

__all__ = ("find_redbot_update_bin",)


def find_redbot_update_bin() -> str:
    binary_name = "redbot-update" + sysconfig.get_config_var("EXE")

    path = os.path.join(sysconfig.get_path("scripts"), binary_name)
    if os.path.isfile(path):
        return path

    raise FileNotFoundError(path)
