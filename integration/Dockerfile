# amd64 is deliberate: codec versions and CPU architecture are part of the recipe.
FROM ubuntu:24.04@sha256:78d4744ead1f5b19b04d0df558ab182a5300903900755e515b2ca4d10eb2044d
ENV DEBIAN_FRONTEND=noninteractive LC_ALL=C TZ=UTC OMP_NUM_THREADS=1 SOURCE_DATE_EPOCH=0
# Freeze the package repository, not just the distribution name.
RUN rm -f /etc/apt/sources.list /etc/apt/sources.list.d/* \
    && echo 'deb http://snapshot.ubuntu.com/ubuntu/20260828T000000Z/ noble main universe' > /etc/apt/sources.list \
    && echo 'deb http://snapshot.ubuntu.com/ubuntu/20260828T000000Z/ noble-updates main universe' >> /etc/apt/sources.list \
    && echo 'deb http://snapshot.ubuntu.com/ubuntu/20260828T000000Z/ noble-security main universe' >> /etc/apt/sources.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends python3 python3-venv ca-certificates \
       imagemagick gimp farbfeld icoutils libjxl-tools libavif-bin libheif-examples \
       libheif-plugin-x265 libheif-plugin-libde265 \
    && rm -rf /var/lib/apt/lists/*
COPY integration/requirements.txt /tmp/requirements.txt
RUN python3 -m venv /opt/fixtures \
    && /opt/fixtures/bin/pip install --require-hashes --no-cache-dir --only-binary=:all: -r /tmp/requirements.txt
ENV PATH=/opt/fixtures/bin:$PATH
WORKDIR /repo
ENTRYPOINT ["python3", "integration/generate.py"]
