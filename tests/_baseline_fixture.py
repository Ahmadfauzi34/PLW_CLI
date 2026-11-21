"""Shared isolated fixture for PLW architecture & calibration baseline suites."""
from __future__ import annotations

import os
import shutil
import tempfile
import unittest
from pathlib import Path


class ArchitectureBaselineCase(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = Path(tempfile.mkdtemp(prefix="plw_arch_base_"))
        self._prev_plw_home = os.environ.get("PLW_HOME")
        os.environ["PLW_HOME"] = str(self._tmp / "plw_home")

        self.proj = self._tmp / "sample_py"
        (self.proj / "src" / "app").mkdir(parents=True)
        (self.proj / "src" / "app" / "__init__.py").write_text(
            "from .router import route\n", encoding="utf-8"
        )
        (self.proj / "src" / "app" / "router.py").write_text(
            "from app.service import handle\nfrom .utils import fmt\n\n"
            "def route(x):\n    return fmt(handle(x))\n",
            encoding="utf-8",
        )
        (self.proj / "src" / "app" / "service.py").write_text(
            "from .utils import HelperState, fmt\n\n"
            "def handle(x):\n    return fmt(HelperState.VALUE)\n",
            encoding="utf-8",
        )
        (self.proj / "src" / "app" / "utils.py").write_text(
            "from app.service import handle\n\n"
            "class HelperState:\n    VALUE = 'ok'\n\n"
            "def fmt(v):\n    return str(v)\n",
            encoding="utf-8",
        )
        (self.proj / "main.py").write_text(
            "#!/usr/bin/env python3\nfrom app import route\n\n"
            "if __name__ == '__main__':\n    print(route('go'))\n",
            encoding="utf-8",
        )

    def tearDown(self) -> None:
        if self._prev_plw_home is None:
            os.environ.pop("PLW_HOME", None)
        else:
            os.environ["PLW_HOME"] = self._prev_plw_home
        shutil.rmtree(self._tmp, ignore_errors=True)
