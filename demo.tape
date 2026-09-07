Output demo.gif

Set Shell "bash"
Set FontSize 22
Set Width 1650
Set Height 900
Set Padding 32
Set Theme "Dark+"
Set Framerate 30
Set TypingSpeed 12ms

# ---- Clean, deterministic bootstrap (not shown in the GIF) ----
Hide
Type "rm -rf /tmp/demo && mkdir -p /tmp/demo" Enter
Sleep 200ms
Type "cd /tmp/demo" Enter
Sleep 200ms
Type "clear" Enter
Show

# =====================================================================
# 1. Create a small demo project (directories + files)
# =====================================================================
Sleep 900ms
Type "mkdir -p src/app config docs" Enter
Sleep 900ms
Type "echo 'package app' > src/app/main.go" Enter
Sleep 500ms
Type "echo 'const Version = 1' > src/app/version.go" Enter
Sleep 500ms
Type "echo 'app: true' > config/app.yaml" Enter
Sleep 500ms
Type "echo '# demo notes' > docs/README.md" Enter
Sleep 500ms
Type "echo 'hello from a plain file' > top-level.txt" Enter
Sleep 900ms
Type "tree" Enter
Sleep 3200ms

# =====================================================================
# 2. First snapshot: every change is reported as created (+)
# =====================================================================
Hide
Type "clear" Enter
Show
Sleep 600ms
Type "snapshotter" Enter
Sleep 3800ms

# =====================================================================
# 3. Edit one file, then snapshot again: that file is modified (~)
# =====================================================================
Hide
Type "clear" Enter
Show
Sleep 600ms
Type "echo 'package app // second version' > src/app/main.go" Enter
Sleep 900ms
Type "snapshotter" Enter
Sleep 3800ms

# =====================================================================
# 4. Peek at the mirrored .snapshots layout
# =====================================================================
Hide
Type "clear" Enter
Show
Sleep 600ms
Type "tree .snapshots" Enter
Sleep 4200ms

# =====================================================================
# 5. Delete a file, snapshot again: that file is deleted (-)
# =====================================================================
Hide
Type "clear" Enter
Show
Sleep 600ms
Type "rm top-level.txt" Enter
Sleep 900ms
Type "snapshotter" Enter
Sleep 3800ms

# =====================================================================
# 6. Inspect the append-only event log with jq
# =====================================================================
Hide
Type "clear" Enter
Show
Sleep 600ms
Type "jq -c '.changes[] | {action, path}' .snapshots/.internal/events.jsonl" Enter
Sleep 4200ms

# =====================================================================
# 7. Move a file, snapshot again: that file is moved (>)
# =====================================================================
Hide
Type "clear" Enter
Show
Sleep 600ms
Type "mkdir lib" Enter
Sleep 600ms
Type "mv src/app/version.go lib/version.go" Enter
Sleep 900ms
Type "snapshotter" Enter
Sleep 3800ms

# =====================================================================
# 8. Tail the ledger and pretty-print the newest event
# =====================================================================
Hide
Type "clear" Enter
Show
Sleep 600ms
Type "tail -n 1 .snapshots/.internal/events.jsonl | jq '.changes[0]'" Enter
Sleep 4600ms

# =====================================================================
# 9. Roll the edited file back to its original (interactive viewer)
# =====================================================================
Hide
Type "clear" Enter
Show
Sleep 600ms
Type "snapshotter src/app/main.go" Enter
# Wait for the full-screen TUI to paint
Sleep 3200ms
# Move the cursor onto the earlier (created) version and restore it
Down
Sleep 900ms
Enter
Sleep 4200ms
# Quit the viewer back to the shell
Type "q"
Sleep 1000ms
# Show the restored file content
Type "cat src/app/main.go" Enter
Sleep 3200ms
