# Lurpic Studio

A full-marks-showcase demo for lurpicUI — a mock data-dashboard builder that
exercises every mark in the standard collection (9 visual families, 48 concrete
marks).

## Run on desktop

```bash
go run ./demos/lurpic_studio
```

## Run on Android

```bash
lurpic build android --project demos/lurpic_studio
lurpic run android --emulator --project demos/lurpic_studio
```

## Manual-QA checklist

48 gestures covering every mark — see the implementation spec §6 Coverage Matrix.
