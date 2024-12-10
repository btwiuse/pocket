FROM btwiuse/ufo AS builder-ufo

FROM btwiuse/arch:node AS builder-node

COPY . /pocket

WORKDIR /pocket

RUN make frontend

FROM btwiuse/arch:golang AS builder-golang

COPY --from=builder-node /pocket /pocket

WORKDIR /pocket

ENV GONOSUMDB="*"

RUN make

FROM btwiuse/arch

COPY --from=builder-ufo /usr/bin/ufo /usr/bin/ufo

COPY --from=builder-golang /pocket/pocket /usr/bin/pocket
