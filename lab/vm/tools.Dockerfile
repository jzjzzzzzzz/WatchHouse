FROM ubuntu:24.04@sha256:534baea6a22c03a63003dbc8dbe78fe34bc0d7e595d9a9dc9834884ff530eb55
LABEL org.watchhouse.component="lab-vm-tools"
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install --no-install-recommends -y \
    qemu-system-arm qemu-utils qemu-efi-aarch64 cloud-image-utils ca-certificates \
    && apt-get clean
CMD ["qemu-system-aarch64", "--version"]
