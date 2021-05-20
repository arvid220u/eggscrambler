#!/usr/bin/env python3

"""
log.py prettifies a log, making it infinitely easier to debug.
Inspired by this blogpost: https://blog.josejg.com/debugging-pretty/
"""

from typing import Optional
import typer
from rich import print
from rich.console import Console
from rich.columns import Columns
import re
import sys

TOPICS = {
    "INFO": "bright_white",
    "WARN": "bright_yellow",
    "ERRO": "red",
    "DUMP": "red",
}


def stripleaks(alines):
    newalines = []
    init = False
    for aline in alines:
        if "==================" in aline["line"]:
            init = not init
        elif not init:
            newalines.append(aline)
    return newalines


def colorstuff(alines):
    for aline in alines:
        if "topic" in aline:
            color = TOPICS[aline["topic"]]
            aline["line"] = f"[{color}]" + aline["line"] + f"[/{color}]"
    return alines


def main(fn: Optional[str] = None, cwidth: Optional[int] = None, color: bool = True):
    if cwidth is None:
        console = Console(force_terminal=color, highlight=False)
    else:
        console = Console(force_terminal=color, highlight=False, width=cwidth)

    lines = []
    if fn is None:
        for line in sys.stdin:
            lines.append(line)
    else:
        with open(fn, "r") as f:
            lines = f.readlines()

    annotated_lines = []
    ids = set()
    for line in lines:
        line = line.strip("\n")
        annotated_line = {"line": line}

        rgx = r"(\d{2}:\d{2}:\d{2}.\d{6}) ([A-Z]{4}) \[server (\d+)\](.*)"
        m = re.match(rgx, line)
        if m:
            annotated_line["type"] = "SERVER"
            annotated_line["time"] = m.group(1)
            annotated_line["topic"] = m.group(2)
            annotated_line["server"] = int(m.group(3))
            annotated_line["id"] = "S" + m.group(3)
            annotated_line["line"] = m.group(4)
            ids.add(annotated_line["id"])

        rgx_client = r"(\d{2}:\d{2}:\d{2}.\d{6}) ([A-Z]{4}) \[client ([a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}) \(on server (\d+)\)\](.*)"
        m = re.match(rgx_client, line)
        if m:
            annotated_line["type"] = "CLIENT"
            annotated_line["time"] = m.group(1)
            annotated_line["topic"] = m.group(2)
            annotated_line["client"] = m.group(3)
            annotated_line["server"] = int(m.group(4))
            annotated_line["line"] = m.group(5)
            annotated_line["id"] = "C" + annotated_line["client"][:8]
            ids.add(annotated_line["id"])

        if "type" not in annotated_line:
            annotated_line["type"] = "SPECIAL"

        annotated_line["line"] = annotated_line["line"].replace("[", "\[")
        annotated_lines.append(annotated_line)

    # annotated_lines = stripleaks(annotated_lines)
    annotated_lines = colorstuff(annotated_lines)

    ids_list = list(ids)
    for aline in annotated_lines:
        if aline["type"] == "SPECIAL":
            continue
        aline["id_index"] = ids_list.index(aline["id"])

    width = console.size.width
    n_columns = max(len(ids), 1)
    col_width = width // n_columns - 1

    for aline in annotated_lines:
        line = aline["line"]
        if aline["type"] == "SPECIAL":
            console.print(f"[green]{line}[/green]")
            continue

        i = aline["id_index"]
        cols = [" " for _ in range(n_columns)]
        cols[
            i
        ] = f"[magenta]{aline['id']}[/magenta]@[bright_blue]{aline['time']}[/bright_blue]: {line}"
        c = Columns(cols, width=col_width, equal=True, expand=True)
        console.print(c)


if __name__ == "__main__":
    typer.run(main)
