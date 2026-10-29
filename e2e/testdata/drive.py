#!/usr/bin/env python3
"""Drive a real Vim or Neovim in a pseudo terminal.

usage: drive.py EDITOR VIMRC STEPS_JSON

STEPS_JSON is a list of [keys, seconds to wait after them]. The pty is
drained all the time, so that the editor never blocks on its output.
"""
import fcntl, json, os, pty, select, struct, sys, termios, time


def main():
    editor, vimrc, steps = sys.argv[1], sys.argv[2], json.loads(sys.argv[3])
    pid, fd = pty.fork()
    if pid == 0:
        os.environ["TERM"] = "xterm-256color"
        os.execvp(editor, [editor, "-u", vimrc, "-i", "NONE", "-N", "-n"])
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 30, 100, 0, 0))

    def drain(seconds):
        end = time.time() + seconds
        while time.time() < end:
            r, _, _ = select.select([fd], [], [], 0.05)
            if r:
                try:
                    if not os.read(fd, 65536):
                        return False
                except OSError:
                    return False
        return True

    alive = drain(1.5)
    for keys, wait in steps:
        if not alive:
            break
        os.write(fd, keys.encode())
        alive = drain(wait)
    if alive:
        os.write(fd, b"\x1b:qa!\r")
        drain(1.0)
    try:
        os.kill(pid, 9)
    except OSError:
        pass
    os.waitpid(pid, 0)


main()
