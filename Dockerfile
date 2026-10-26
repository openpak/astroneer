# syntax=docker/dockerfile:1
# ASTRONEER's edge service for the Switch build (0100E63013E60000).
# Plain HTTP on 23560 behind the TLS terminator; health on 23561.
# NEX itself is served by switch-nex (port base 22200); PlayFab by the playfab shim.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOWORK=off go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/astroneerd ./cmd/astroneerd

FROM gcr.io/distroless/static
COPY --from=build /out/astroneerd /astroneerd
USER nonroot:nonroot
EXPOSE 23560 23561
ENTRYPOINT ["/astroneerd"]
