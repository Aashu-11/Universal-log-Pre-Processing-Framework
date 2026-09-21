# Extends the official Apache Hive standalone-metastore image with the
# PostgreSQL JDBC driver. The driver is fetched HERE, at image BUILD time
# only (`docker compose build`, cached thereafter) — never at container run
# time — so the running system makes zero outbound calls, per CLAUDE.md's
# air-gap rule. sha256 is pinned and verified so a tampered download fails
# the build loudly instead of installing a bad driver.
FROM apache/hive:4.0.0

ARG PG_JDBC_VERSION=42.7.3
ARG PG_JDBC_SHA256=a2644cbfba1baa145ff7e8c8ef582a6eed7a7ec4ca792f7f054122bdec756268

USER root
# apache/hive:4.0.0's Debian bullseye base has neither curl nor an
# unexpired bullseye-security Release file (the base image is old enough
# that its baked-in apt metadata has since expired) — install curl with
# Check-Valid-Until disabled for this one `apt-get update`, which only
# affects trusting the repository's freshness claim, not package
# signatures (packages are still GPG-verified normally).
RUN printf 'deb http://archive.debian.org/debian bullseye main contrib non-free\ndeb http://archive.debian.org/debian bullseye-backports main contrib non-free\n' > /etc/apt/sources.list \
    && rm -f /etc/apt/sources.list.d/*.list \
    && apt-get -o Acquire::Check-Valid-Until=false update \
    && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && curl -sSL -o /opt/hive/lib/postgresql-jdbc.jar \
      "https://repo1.maven.org/maven2/org/postgresql/postgresql/${PG_JDBC_VERSION}/postgresql-${PG_JDBC_VERSION}.jar" \
    && echo "${PG_JDBC_SHA256}  /opt/hive/lib/postgresql-jdbc.jar" | sha256sum -c -

# The S3A connector (hadoop-aws + its bundled AWS SDK) ships inside this
# same image already, under Hadoop's optional "tools" lib dir — but that
# directory isn't on the Hive Metastore's classpath by default, which is
# why hive.metastore.warehouse.dir=s3a://... fails with
# ClassNotFoundException: S3AFileSystem until these two jars are copied
# into /opt/hive/lib (no network fetch needed, both files already exist in
# the base image).
RUN cp /opt/hadoop/share/hadoop/tools/lib/hadoop-aws-*.jar /opt/hive/lib/ \
    && cp /opt/hadoop/share/hadoop/tools/lib/aws-java-sdk-bundle-*.jar /opt/hive/lib/
USER hive
