#!/usr/bin/env python3

import typer
from typing import Optional
from rich import print
import os
import tempfile
import subprocess
from rich.markup import escape
from rich.console import Console
import random

c = Console(highlight=False)
def print(s):
    c.print(s)


def run(s: Optional[str] = None, race: bool = True) -> bool:
    success = True
    if s is None:
        p = subprocess.Popen(['go', 'test', '-count=1', '-race' if race else ''],stdout=subprocess.PIPE)
    else:
        p = subprocess.Popen(['go', 'test', '-run', s, '-count=1', '-race' if race else ''],stdout=subprocess.PIPE)
    output = ""
    for line in iter(p.stdout.readline, b''):
        out = line.decode('utf-8')
        output += out
        out = out.strip("\n")
        if "PASS" in out:
            print(f"[green]{escape(out)}[/green]")
        elif "FAIL" in out:
            print(f"[red]{escape(out)}[/red]")
            success = False
        else:
            print(escape(out))
    if not success:
        fn = f"{s}-fail-{random.randint(1,10000)}"
        print(f"[magenta]saving failed log file to {fn}")
        with open(fn, "w") as f:
            f.write(output)
    return success

def main(n: int, testname: Optional[str] = None, race: bool = True):
    success = True
    for i in range(n):
        print(f"[yellow]Running test {i+1} of {n}[/yellow]")
        if testname is not None:
            success = success and run(testname, race=race)
        else:
            success = success and run(race=race)
        if not success:
            break
    if success:
        print("[green bold]YAYAYAY EVERYTHING WORKS")

typer.run(main)
