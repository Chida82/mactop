# mactop (fork Chida82) — specifica

Fork di [metaspartan/mactop](https://github.com/metaspartan/mactop) (Go + Objective-C via cgo, MIT)
con un layout personale, `chida`. Tutto il resto resta identico a upstream, per continuare a prendere i fix.

## Repo

- `origin` = `Chida82/mactop` (fork **pubblico**), `upstream` = `metaspartan/mactop`.
- `main` resta allineato a upstream; il lavoro sta sul ramo `layout-chida`.

## Prendere le correzioni dell'autore (upstream)

```
cd ~/github/chida82/mactop
git fetch upstream
git switch main && git merge --ff-only upstream/main && git push origin main   # main = specchio di upstream
git switch layout-chida && git rebase main                                       # i nostri commit sopra
go build -o mactop . && go test ./internal/app/ ./internal/i18n/
git push --force-with-lease origin layout-chida                                  # il rebase riscrive il ramo
```

- `main` non deve mai avere commit nostri: se `--ff-only` fallisce, qualcosa è finito su `main` per errore.
- Conflitti: possibili solo su `layout_chida.go`/`CLAUDE.md` se upstream crea file con lo stesso nome (improbabile).
  Più probabile che il build fallisca perché upstream ha rinominato qualcosa che usiamo:
  `layoutOrder`, `namedLayoutSetters`, `grid`, `cpuHistoryChart`, `gpuHistoryChart`, `memBWHistoryChart`,
  `memoryHistoryChart`, `cpuGauge`, `gpuGauge`, `NetworkInfo`, `PowerChart`, `sparklineGroup`, `processList`,
  `lastCPUMetrics`, `buildFanStatusText`, `buildGroupedTempLines`, `formatTemp`, `currentConfig`, `IsLightMode`.
  Si sistema solo `layout_chida.go`.
- Dopo ogni aggiornamento riprovare il layout a schermo (vedi "Build e prova"): un upstream che cambia
  i widget può compilare ma disegnare male.

## Regola principale: non toccare i file upstream

Il layout vive solo in `internal/app/layout_chida.go` e si registra da solo in `init()`
(`layoutOrder` in testa, `namedLayoutSetters`). Diff contro upstream = solo quel file + questo.
Se serve cambiare comportamento upstream, preferire un wrapper nel nostro file.

## Layout `chida` (primo dell'elenco, tasto `l`)

Basato su `vertical` (L4 originale):

```
colonna sinistra 40%                         colonna destra 60%
CPU   grafico storico   1.25/8               Processi                     0.68
GPU   grafico storico   1.25/8
DRAM  lettura/scrittura 1.0/8
MEM   usata + swap      1.5/8
Network & Disk          1.0/8
Power | sparkline watt  2.0/8                Ventole | Temperature        0.32
```

- **CPU/GPU**: `cpuHistoryChart`/`gpuHistoryChart` con il titolo preso da `cpuGauge.Title`/`gpuGauge.Title`
  (uso, frequenza, temperatura), così bastano poche righe.
- **DRAM**: `memBWHistoryChart` (al posto di ANE), titolo con R e W. **MEM**: `memoryHistoryChart` (due linee, usata e swap, come L10).
- Grafici a due linee: i temi standard forzano un colore unico (`styleStepChart`), quindi `tailChart` colora la
  seconda linea in magenta (giallo se il tema è magenta) e mette il nome della serie davanti all'etichetta (`R`/`W`, `Usata`/`Swap`).
- **Consumi**: `PowerChart` + `sparklineGroup` spostati in basso a sinistra.
- **Ventole**: contenuto di `buildFanStatusText` (come il layout fan); titolo con lo stato fanboost
  (`Mode == 1` → "BOOST (manuale)", altrimenti "AUTO (curva Apple)").
- **Temperature**: prima riga con i massimi usati da fanboost (CPU `Tp*`/`Te*`, GPU `Tg*`, MEM `Tm*`,
  SSD da NVMe `Nv*`, BAT `TB0T/TB1T/TB2T`), poi `buildGroupedTempLines` come nel layout fan.
  Il gruppo "SSD" di upstream sono chiavi `TS*`, non l'SSD: il valore giusto è NVMe.

## Dettagli implementativi

- `tailChart`: StepChart disegna da sinistra e gli update upstream tagliano la storia sulla larghezza
  dei loro layout; il wrapper tiene solo gli ultimi `Inner.Dx()` punti, altrimenti si vedrebbe la parte vecchia.
- `textPanel`: rigenera titolo e testo a ogni Draw, con lo stile del pannello rete (segue il tema).
- Fan e sensori sono campionati sempre da upstream (`lastCPUMetrics.Fans`, `.TempSensors`), anche fuori dal layout fan.

## Build e prova

- `go build -o mactop .` (Go ≥ 1.25); test: `go test ./internal/app/ ./internal/i18n/`.
- Installazione: `./install-chida.sh` (build + `sudo install` in `/usr/local/bin/mactop`, `root:wheel` 755).
  Script separato apposta: il `Makefile` è di upstream. Se si reinstalla mactop con Homebrew,
  `/opt/homebrew/bin` viene prima nel PATH e nasconde questa versione.
- Prova visiva senza toccare `~/.mactop/config.json`: config isolata con
  `XDG_CONFIG_HOME=<dir>` contenente `mactop/config.json` con `"default_layout": "chida"`,
  lanciata in un tab Herdr separato e letta con `herdr pane read <pane> --source visible --format text`.

## Idee aperte

- Zoom della storia (1m / 10m / 1h) a parità di frequenza: buffer di 1 ora e colonne raggruppate al **massimo**
  (non la media, per non perdere i picchi). Candidato a PR upstream.
