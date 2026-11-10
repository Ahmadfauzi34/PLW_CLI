"""Standalone portable-agent discovery and readiness surface for PLW.

This module exposes only self-description, embedded skill lookup, and readiness
checks. It does not grant execution, mutation, promotion, or truth authority.
"""
from __future__ import annotations

import os
import shutil
import sys
from pathlib import Path
from typing import Any, Dict, Optional

from core.shared_graph import DEFAULT_IGNORE_DIRS, discover_source_files
from core.semantic_affordance import COMMAND_CONTRACTS, describe_command
from language_semantics import JS_SOURCE_EXTENSIONS

SCHEMA_VERSION = "plw-portable-agent-v1"
POLICY = "standalone-binary-target-first-agent-surface-v1"

DYNAMIC_CONTEXT_SCHEMA_VERSION = "plw-dynamic-context-v1"

# Languages the dynamic context matrix distinguishes, by file extension.
# language_semantics only defines JS/Python sets; Go/Rust live here because
# the matrix (not the stable topology graph) is what needs them.
MATRIX_LANGUAGE_EXTENSIONS = {
    ".go": "go",
    ".rs": "rust",
    ".py": "python",
    ".js": "javascript",
    ".jsx": "jsx",
    ".ts": "typescript",
    ".tsx": "tsx",
}
# Languages the stable topology graph covers without any engine.
STABLE_TOPOLOGY_LANGUAGES = {"javascript", "jsx", "typescript", "tsx"}
# Languages whose structural questions need the tree-sitter engine.
ENGINE_LANGUAGES = {"go", "rust", "python"}


def _state_root() -> Path:
    home = os.environ.get("PLW_HOME")
    if home:
        return Path(home).expanduser() / "state"
    xdg = os.environ.get("XDG_STATE_HOME")
    if xdg:
        return Path(xdg).expanduser() / "plw"
    return Path.home().expanduser() / ".local" / "state" / "plw"


def _cache_root() -> Path:
    home = os.environ.get("PLW_HOME")
    if home:
        return Path(home).expanduser() / "cache"
    xdg = os.environ.get("XDG_CACHE_HOME")
    if xdg:
        return Path(xdg).expanduser() / "plw"
    return Path.home().expanduser() / ".cache" / "plw"


def _resource_root() -> Path:
    frozen = getattr(sys, "_MEIPASS", None)
    if frozen:
        return Path(str(frozen)).resolve()
    return Path(__file__).resolve().parent.parent


def _skill_root() -> Path:
    return _resource_root() / "skills"


def build_version() -> str:
    candidates = [
        _resource_root() / "VERSION",
        Path(__file__).resolve().parent.parent / "VERSION",
    ]
    for path in candidates:
        try:
            value = path.read_text(encoding="utf-8").strip()
        except OSError:
            continue
        if value:
            return value
    return "unknown"


def _referenced_skill_paths() -> list[str]:
    return sorted({
        str(contract.get("read_before_use"))
        for contract in COMMAND_CONTRACTS.values()
        if contract.get("read_before_use")
    })


def _safe_skill_path(skill_path: str) -> Optional[Path]:
    normalized = str(skill_path or "").strip().replace("\\", "/")
    if not normalized:
        return None
    if normalized in COMMAND_CONTRACTS:
        normalized = str(COMMAND_CONTRACTS[normalized].get("read_before_use") or "")
    elif not normalized.startswith("skills/"):
        if normalized.endswith(".md"):
            normalized = "skills/" + normalized.lstrip("/")
        else:
            matches = [
                path for path in _referenced_skill_paths()
                if Path(path).stem == normalized or Path(path).name == normalized
            ]
            if len(matches) == 1:
                normalized = matches[0]
    if not normalized.startswith("skills/"):
        return None
    relative = Path(normalized).relative_to("skills")
    if any(part in ("", ".", "..") for part in relative.parts):
        return None
    root = _skill_root().resolve()
    candidate = (root / relative).resolve()
    try:
        candidate.relative_to(root)
    except ValueError:
        return None
    return candidate


def list_capabilities() -> Dict[str, Any]:
    rows = []
    for command in sorted(COMMAND_CONTRACTS):
        contract = COMMAND_CONTRACTS[command]
        skill_path = str(contract.get("read_before_use") or "")
        skill_file = _safe_skill_path(skill_path)
        rows.append({
            "command": command,
            "canonical_concepts": list(contract.get("canonical_concepts", []) or []),
            "skill_path": skill_path,
            "skill_embedded": bool(skill_file and skill_file.is_file()),
            "state_change": bool(contract.get("state_change", False)),
        })
    return {
        "schema_version": SCHEMA_VERSION,
        "policy": POLICY,
        "capability_count": len(rows),
        "capabilities": rows,
        "authority": {
            "descriptive_only": True,
            "executes_capability": False,
            "mutate_source": False,
            "commit_truth": False,
        },
    }


def describe_capability(command: str) -> Dict[str, Any]:
    result = dict(describe_command(command))
    result["portable_agent_surface"] = SCHEMA_VERSION
    result["embedded_skill_available"] = bool(
        _safe_skill_path(str(result.get("read_before_use") or ""))
        and _safe_skill_path(str(result.get("read_before_use") or "")).is_file()
    )
    return result


def list_skills() -> Dict[str, Any]:
    root = _skill_root()
    paths = sorted(root.glob("*.md")) if root.is_dir() else []
    referenced = set(_referenced_skill_paths())
    rows = []
    for path in paths:
        skill_path = "skills/" + path.name
        rows.append({
            "skill_path": skill_path,
            "embedded": True,
            "referenced_by_capability": skill_path in referenced,
        })
    missing = sorted(path for path in referenced if not (_safe_skill_path(path) or Path()).is_file())
    return {
        "schema_version": SCHEMA_VERSION,
        "policy": POLICY,
        "skill_count": len(rows),
        "embedded_count": len(rows),
        "referenced_skill_count": len(referenced),
        "missing_referenced_skills": missing,
        "skills": rows,
        "authority": "documentation_discovery_only",
    }


def read_skill(value: str) -> Dict[str, Any]:
    path = _safe_skill_path(value)
    if path is None or not path.is_file():
        return {
            "schema_version": SCHEMA_VERSION,
            "error": f"embedded skill not found: {value}",
            "known": False,
        }
    content = path.read_text(encoding="utf-8")
    skill_path = "skills/" + path.name
    commands = sorted(
        command
        for command, contract in COMMAND_CONTRACTS.items()
        if contract.get("read_before_use") == skill_path
    )
    return {
        "schema_version": SCHEMA_VERSION,
        "known": True,
        "skill_path": skill_path,
        "commands": commands,
        "content": content,
        "authority": "documentation_read_only",
    }


def _path_is_within(path: Path, root: Path) -> bool:
    try:
        path.resolve().relative_to(root.resolve())
        return True
    except (OSError, ValueError):
        return False


def _browser() -> Dict[str, Any]:
    for name in (
        "chromium",
        "chromium-browser",
        "google-chrome",
        "google-chrome-stable",
        "chrome",
    ):
        path = shutil.which(name)
        if path:
            return {"available": True, "executable": path, "required_for_static": False}
    return {"available": False, "executable": None, "required_for_static": False}


def _codesearch_engine_status() -> Dict[str, Any]:
    """Tree-sitter engine availability for `plw doctor`. Never raises.

    The engine module is imported lazily so a missing or broken engine
    can never take `doctor` (or the rest of the CLI) down with it:
    doctor reports the engine as unavailable instead.
    """
    try:
        from core.codesearch_engine import doctor_status
    except Exception:
        return {
            "available": False,
            "tree_sitter_ready": False,
            "binary": None,
            "discovered_via": None,
            "schema": None,
            "commit": None,
            "languages": [],
            "reason": "codesearch engine module unavailable",
        }
    return doctor_status()


def _target_languages(root: str = ".") -> Dict[str, int]:
    """Count target files per matrix language. Never raises.

    Same walk discipline as discover_source_files (ignored dirs, no
    hidden dirs) but keyed by matrix language instead of stable scope,
    so Go/Rust/Python files — invisible to the topology graph — are
    counted here.
    """
    target = Path(root).expanduser().resolve()
    counts: Dict[str, int] = {}
    if not target.is_dir():
        return counts
    try:
        for dirpath, dirnames, filenames in os.walk(target):
            dirnames[:] = sorted(
                d
                for d in dirnames
                if d not in DEFAULT_IGNORE_DIRS and not d.startswith(".")
            )
            for filename in sorted(filenames):
                language = MATRIX_LANGUAGE_EXTENSIONS.get(
                    Path(filename).suffix.lower()
                )
                if language is None:
                    continue
                counts[language] = counts.get(language, 0) + 1
    except OSError:
        return {}
    return counts


def dynamic_context(root: str = ".") -> Dict[str, Any]:
    """Execute the dynamic context matrix (KONTEKS_DAN_RILIS.md §2) for one target root.

    Inputs, all computed from the target — never from the task text:
    the kernel's capability selection is explicitly not keyword-based
    (policy evidence-class-to-capability-contract-not-task-keyword-v1),
    so no task kind is classified here. Instead the matrix reports
    which routes exist for this (languages, engine) state, and the
    agent picks the route matching its task:

    * target languages: Go/Rust/Python need the engine; TS/JS/JSX/TSX
      are answered by the stable topology graph alone.
    * engine availability: from doctor (available -> context B,
      unavailable -> context A plus the recorded reason).
    * routes: structural questions go via `plw ast` when the engine is
      present, via the stable graph when the target is TS/JS-only, and
      nowhere when the target needs the engine and it is missing.
      Topology stays TS/JS-only either way (G6: the engine does not
      extend topology).
    """
    languages = _target_languages(root)
    engine = _codesearch_engine_status()
    support = target_topology_support(root)
    needs_engine = any(lang in ENGINE_LANGUAGES for lang in languages)
    engine_available = bool(engine.get("available"))
    context = "B" if engine_available else "A"

    if not languages:
        structural: Dict[str, Any] = {
            "available": False,
            "route": None,
            "reason": "no matrix-language files found in target",
        }
    elif engine_available:
        structural = {
            "available": True,
            "route": "plw ast defs|callers|related|search <root> <query>",
            "reason": "engine available",
        }
    elif not needs_engine:
        structural = {
            "available": True,
            "route": "stable topology graph (plw topology)",
            "reason": "target languages are all in stable topology scope",
        }
    else:
        structural = {
            "available": False,
            "route": None,
            "reason": (
                "engine unavailable; structural questions on go/rust/python "
                "need the codesearch-go release binary"
            ),
        }
    topology: Dict[str, Any] = {
        "available": bool(support.get("topology_ready")),
        "scope": "stable_shared_graph (.ts/.tsx/.js/.jsx only)",
        "reason": str(support.get("reason") or ""),
        "note": "the engine does not extend topology (G6)",
    }

    if not languages:
        reason = "Context " + context + ": no matrix-language files found in target."
    elif context == "B":
        engine_id = ""
        if engine.get("schema"):
            engine_id = f" (schema {engine.get('schema')}, commit {engine.get('commit')})"
        reason = (
            f"Context B: engine available{engine_id}; "
            "structural questions answered via plw ast; "
            "topology stays TS/JS-only."
        )
    elif needs_engine:
        missing = sorted(lang for lang in languages if lang in ENGINE_LANGUAGES)
        parts = ", ".join(
            f"{lang} ({languages[lang]} file{'s' if languages[lang] != 1 else ''})"
            for lang in missing
        )
        engine_reason = f" ({engine.get('reason')})" if engine.get("reason") else ""
        reason = (
            f"Context A: target contains {parts} which need the engine; "
            f"engine unavailable{engine_reason}; "
            "structural questions cannot be answered; "
            "required: codesearch-go release binary."
        )
    else:
        reason = (
            "Context A: target languages are all in stable topology scope; "
            "no engine needed."
        )
    return {
        "schema_version": DYNAMIC_CONTEXT_SCHEMA_VERSION,
        "context": context,
        "languages": languages,
        "needs_engine": needs_engine,
        "engine": engine,
        "topology": topology,
        "structural_query": structural,
        "reason": reason,
    }


def target_topology_support(root: str = ".") -> Dict[str, Any]:
    """Describe the stable shared-graph source scope for one target root."""
    target = Path(root).expanduser().resolve()
    if not target.is_dir():
        supported = 0
        reason = "TARGET_ROOT_UNAVAILABLE"
    else:
        supported = len(discover_source_files(str(target), include_tool_source=False))
        reason = (
            "STABLE_TOPOLOGY_SOURCE_FILES_FOUND"
            if supported
            else "NO_STABLE_TOPOLOGY_SOURCE_FILES"
        )
    return {
        "scope": "stable_shared_graph",
        "topology_extensions": list(JS_SOURCE_EXTENSIONS),
        "supported_source_files": supported,
        "topology_ready": bool(supported),
        "reason": reason,
        "authority": {
            "stable_scope_only": True,
            "experimental_python_promoted": False,
        },
    }


def doctor(root: str = ".") -> Dict[str, Any]:
    target = Path(root).expanduser().resolve()
    version = build_version()
    caps = list_capabilities()
    skills = list_skills()
    state_root = _state_root().expanduser().resolve()
    cache_root = _cache_root().expanduser().resolve()
    target_support = target_topology_support(str(target))
    checks = {
        "target_root_exists": target.is_dir(),
        "embedded_version": version != "unknown",
        "capability_registry": caps.get("capability_count", 0) > 0,
        "embedded_skills": skills.get("skill_count", 0) > 0,
        "referenced_skills_complete": not skills.get("missing_referenced_skills"),
        "runtime_state_outside_target": not _path_is_within(state_root, target),
        "runtime_cache_outside_target": not _path_is_within(cache_root, target),
    }
    ready = all(checks.values())
    return {
        "schema_version": SCHEMA_VERSION,
        "policy": POLICY,
        "ready": ready,
        "readiness": {
            "portable_surface_ready": ready,
            "ready_scope": "portable_interface_and_target_root_only",
            "stable_target_topology_ready": ready and bool(target_support["topology_ready"]),
            "stable_target_topology_reason": target_support["reason"],
            "action_readiness": "NOT_ASSESSED",
        },
        "version": version,
        "target_root": str(target),
        "checks": checks,
        "target_support": target_support,
        "runtime": {
            "state_root": str(state_root),
            "cache_root": str(cache_root),
            "browser": _browser(),
            "codesearch_engine": _codesearch_engine_status(),
        },
        "agent": {
            "first_contact": "plw agent orient <root> --task <task> --json",
            "direct_structural_first_contact": "plw topology <root> --json",
            "capability_discovery": "plw capability list --json",
            "skill_discovery": "plw skill list --json",
        },
        "authority": {
            "doctor_executes_runtime": False,
            "doctor_mutates_target": False,
            "doctor_grants_mutation": False,
            "commit_truth": False,
        },
    }


def orientation_surface() -> Dict[str, Any]:
    return {
        "schema_version": SCHEMA_VERSION,
        "entrypoint": "plw agent orient",
        "stable_topology": {
            "extensions": list(JS_SOURCE_EXTENSIONS),
            "experimental_python_promoted": False,
        },
        "flow": [
            "target_topology",
            "target_resolution",
            "evidence_need_derivation",
            "capability_candidates",
            "exact_skill_handoff",
        ],
        "authority": {
            "advisory_only": True,
            "execute_runtime": False,
            "mutate_source": False,
            "mutate_external_repository": False,
            "stable_promotion": False,
            "commit_truth": False,
        },
    }


__all__ = [
    "SCHEMA_VERSION",
    "POLICY",
    "build_version",
    "list_capabilities",
    "describe_capability",
    "list_skills",
    "read_skill",
    "doctor",
    "target_topology_support",
    "dynamic_context",
    "orientation_surface",
]
