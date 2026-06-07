load helpers

IT="$DOCS_REPO_ROOT/docs/toggle/iterm.md"

setup()    { setup_sandbox; need failsafe; }
teardown() { teardown_sandbox; }

@test "shell hook emits OSC 1337 SetUserVar with base64 session id" {
  local hook sid out b64
  hook="$(extract_block "$IT" sh 1)"
  sid="w1t6p0:DEAD-BEEF"
  out="$(ITERM_SESSION_ID="$sid" bash -c "$hook")"
  [[ "$out" == *"1337;SetUserVar=failsafe_sid="* ]]
  b64="${out#*failsafe_sid=}"; b64="${b64%$'\a'}"
  [ "$(printf '%s' "$b64" | base64 -d)" = "$sid" ]
}

@test "doc python read_mode returns canonical and defaults to enabled" {
  [ -n "$PYTHON_BIN" ] || skip "no python"
  extract_block "$IT" python 1 > "$TEST_HOME/it.py"
  run "$PYTHON_BIN" - "$TEST_HOME/it.py" <<'PY'
import ast, sys, os, tempfile
src = open(sys.argv[1]).read()
mod = ast.parse(src)
fn = next(n for n in mod.body
          if isinstance(n, ast.FunctionDef) and n.name == "read_mode")
ns = {}
exec(compile(ast.Module([fn], []), "read_mode", "exec"), ns)
read_mode = ns["read_mode"]
d = tempfile.mkdtemp()
assert read_mode(os.path.join(d, "missing")) == "enabled", "missing file -> enabled"
p = os.path.join(d, "m"); open(p, "w").write("disabled\n")
assert read_mode(p) == "disabled", "canonical preserved"
print("ok")
PY
  [ "$status" -eq 0 ]
  [[ "$output" == *"ok"* ]]
}

@test "doc python script compiles and iterm2 imports" {
  [ -n "$PYTHON_BIN" ] || skip "no python"
  HOME="$ORIG_HOME" "$PYTHON_BIN" -c 'import iterm2' 2>/dev/null || skip "iterm2 not installed"
  extract_block "$IT" python 1 > "$TEST_HOME/it.py"
  run env HOME="$ORIG_HOME" "$PYTHON_BIN" -m py_compile "$TEST_HOME/it.py"
  [ "$status" -eq 0 ]
}

@test "doc python script passes pyflakes" {
  [ -n "$PYTHON_BIN" ] || skip "no python"
  HOME="$ORIG_HOME" "$PYTHON_BIN" -c 'import pyflakes' 2>/dev/null || skip "pyflakes not installed"
  extract_block "$IT" python 1 > "$TEST_HOME/it.py"
  run env HOME="$ORIG_HOME" "$PYTHON_BIN" -m pyflakes "$TEST_HOME/it.py"
  [ "$status" -eq 0 ]
}

@test "no-python alternative: failsafe toggle flips the session file" {
  local sid="w1t6p0:GUID-XYZ"
  ITERM_SESSION_ID="$sid" failsafe toggle
  [ "$(read_mode_file "$sid")" = "disabled" ]
}
