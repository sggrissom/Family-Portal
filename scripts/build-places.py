#!/usr/bin/env python3
"""Rebuilds backend/geodata/cities.tsv.gz from a GeoNames dump.

Usage: build-places.py cities15000.txt admin1CodesASCII.txt countryInfo.txt
(all from https://download.geonames.org/export/dump/).
Each output row: geonameid, name, admin1 name, country name, lat, lon.
"""
import gzip
import sys

cities_path, admin1_path, country_path = sys.argv[1:4]

admin1 = {}
for line in open(admin1_path, encoding="utf-8"):
    code, name, _ascii, _id = line.rstrip("\n").split("\t")
    admin1[code] = name

countries = {}
for line in open(country_path, encoding="utf-8"):
    if line.startswith("#"):
        continue
    cols = line.rstrip("\n").split("\t")
    countries[cols[0]] = cols[4]

rows = []
for line in open(cities_path, encoding="utf-8"):
    c = line.rstrip("\n").split("\t")
    geonameid, name, lat, lon, cc, a1 = c[0], c[1], c[4], c[5], c[8], c[10]
    rows.append("\t".join([
        geonameid, name, admin1.get(f"{cc}.{a1}", ""), countries.get(cc, cc),
        f"{float(lat):.4f}", f"{float(lon):.4f}",
    ]))

rows.sort(key=lambda r: int(r.split("\t", 1)[0]))
with gzip.GzipFile("backend/geodata/cities.tsv.gz", "wb", mtime=0) as out:
    out.write(("\n".join(rows) + "\n").encode("utf-8"))
print(f"{len(rows)} places")
