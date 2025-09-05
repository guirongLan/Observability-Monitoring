# Observability & monitoring

### Avviare client HTTP (/distributed-observability/http-google/client):
```
go run .
```

### Avviare server HTTP in locale (/distributed-observability/http-google/server):
```
go run .
```

### Deployare Server HTTP su google cloud artificial registry

Il server deve essere containerizzato; una volta fatto, si usano i seguenti comandi sempre nello stesso ramo di cartelle del server:

```
docker build -t http-server:latest .

docker tag http-server:latest \
  europe-west8-docker.pkg.dev/organic-cat-465614-m9/http-repository/http-server:v0.4

docker push europe-west8-docker.pkg.dev/organic-cat-465614-m9/http-repository/http-server:v0.4

```
### Seleziona l’immagine dal Google Artifact Registry, scegli l’ambiente di deploy (ad esempio Google Cloud Run) e il processo è completato.