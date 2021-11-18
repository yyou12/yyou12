FROM registry.ci.openshift.org/ocp/builder:rhel-8-golang-1.16-openshift-4.10 AS builder
RUN mkdir -p /go/src/github.com/openshift/openshift-tests-private
WORKDIR /go/src/github.com/openshift/openshift-tests-private
COPY . .
RUN make all; \
    mkdir -p /tmp/build; \
    cp /go/src/github.com/openshift/openshift-tests-private/bin/extended-platform-tests /tmp/build/extended-platform-tests

FROM registry.ci.openshift.org/ocp/4.10:tools
COPY --from=builder /tmp/build/extended-platform-tests /usr/bin/
RUN PACKAGES="git gzip util-linux" && \
    yum install --setopt=tsflags=nodocs -y $PACKAGES && yum clean all && rm -rf /var/cache/yum/* && \
    git config --system user.name test-private && \
    git config --system user.email test-private@test.com && \
    chmod g+w /etc/passwd
RUN pip3 install dotmap minio pyyaml requests
RUN curl -s -k https://gitlab.cee.redhat.com/aosqe/oc-compliance/-/raw/master/oc-compliance.tar -o oc-compliance.tar && tar -C /usr/bin -xvf oc-compliance.tar && rm -fr oc-compliance.tar
RUN curl -s -k https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp-dev-preview/latest-4.10/opm-linux.tar.gz -o opm-linux.tar.gz && tar -C /usr/bin -xzvf opm-linux.tar.gz && rm -fr opm-linux.tar.gz

