#!/usr/bin/env python

from pathlib import Path
from typing import Optional
import subprocess

from jinja2 import Template


def get_schema(dbfile) -> str:
    return subprocess.check_output(["sqlite3", dbfile, ".schema"]).decode("utf8")


def camelize(s: str) -> str:
    return s.replace("_", " ").capitalize().replace(" ", "")


def singularize(s: str) -> str:
    if s.endswith("s"):
        return s[:-1]
    return s


def _typemap(table: Table, typ: str, comment: Optional[str], colname: str) -> str:
    sqltype = typ
    meta = (comment or "").strip().lower()
    if sqltype == "text":
        if meta == "datetime":
            return "time.Time"
        else:
            return "string"
    if sqltype == "json":
        return f"{table.name}_{col.name}"
    if sqltype == "number":
        return "float64"
    raise Exception(f"impossible {sqltype}")


class AST:
    def __init__(self):
        self.nodes: list[Table] = []

    def append(self, node):
        self.nodes.append(node)

    def __repr__(self):
        return f"{self.nodes}\n"

    def __str__(self):
        return self.__repr__()


class Table:
    def __init__(self, name):
        self.name = singularize(camelize(name))
        self.sqlname = name
        self.columns: list[Column] = []

    def append(self, col: "Column"):
        self.columns.append(col)

    def __repr__(self):
        return f"TABLE {self.name}"

    def __str__(self):
        return self.__repr__()


class Column:
    def __init__(
        self, table: Table, name: str, typ: str, attrs: str, comment: Optional[str]
    ):
        self.table = table
        self.name = singularize(camelize(name))
        self.sqlname = name
        self.typ = typ.strip().lower()
        self.gotype = _typemap(table, typ, comment, name)
        self.attrs = attrs
        self.comment = comment

    def __repr__(self):
        return f"COLUMN {self.table.name}.{self.name}: {self.typ} {self.attrs}"

    def __str__(self):
        return self.__repr__()


OPEN = 1
TABLE = 2


def parse(schema: str) -> AST:
    ast = AST()
    state = OPEN
    table = None
    for line in schema.split("\n"):
        if state == OPEN:
            if line.startswith("CREATE TABLE"):
                state = TABLE
                table = Table(line.split(" ")[2])
                print("new table", table)
                continue
        if state == TABLE:
            if line.endswith(");"):
                print("closing table", table)
                ast.append(table)
                table = None
                state = OPEN
                continue
            else:
                comment = None
                if "--" in line:
                    line, comment = line.split("--")
                parts = line.strip().strip(",").split(" ")
                assert table
                column = Column(table, parts[0], parts[1], " ".join(parts[2:]), comment)
                print("new column", column)
                table.append(column)

    return ast


def render(tmpl_file_name, **kwargs) -> str:
    template = Template(open(tmpl_file_name).read())
    return template.render(**kwargs)


def typemap(table: Table, col: Column) -> str:
    sqltype = col.typ.strip().lower()
    meta = (col.comment or "").strip().lower()
    if sqltype == "text":
        if meta == "datetime":
            return "time.Time"
        else:
            return "string"
    if sqltype == "json":
        return f"{table.name}_{col.name}"
    if sqltype == "number":
        return "float64"
    raise Exception(f"impossible {sqltype}")


def write(ast: AST, outdir: Path):
    for table in ast.nodes:
        print(f"{table.name}")
        subtypes = ""
        struct = f"type {table.name} struct {{\n"
        for col in table.columns:
            if (col.comment or "").strip().startswith("{"):
                subtypes += f"type {table.name}_{col.name} struct {col.comment}\n"
            struct += f"	{col.name} {typemap(table, col)}\n"
        struct += "}\n"

        outf = outdir / f"{table.name.lower()}.go"
        open(outf, "w").write(
            render(
                "templates/model.go.jinja",
                table=table,
                struct=struct,
                subtypes=subtypes,
            )
        )
        subprocess.check_output(["goimports", "-w", outf])


if __name__ == "__main__":
    schema = get_schema("../photos.db")
    ast = parse(schema)
    write(ast, Path("out"))
