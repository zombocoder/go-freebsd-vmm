# go-freebsd-vmm
#
# Written for both FreeBSD make and GNU make: no conditionals, all
# branching happens in the shell.

GO?=		go
CC?=		cc
SRCTOP?=	/usr/src
BUILDDIR=	build
LIBSRC=		$(SRCTOP)/lib/libvmmapi

# Strict warnings for the C bridge. These are not put in the cgo CFLAGS
# directive: -Werror there would turn a warning from a future FreeBSD
# system header into a build failure in someone else's project, which is
# not ours to impose. The "cwarn" target keeps the guarantee for us.
CWARNFLAGS=	-Wall -Wextra -Werror

# Linker flags for the local development build (see the libvmmapi
# target). -lutil is needed because a static archive carries no
# transitive dependencies: libvmmapi's vm_parse_memsize() calls
# expand_number(3) from libutil. Linking against the installed shared
# libvmmapi resolves that on its own, so the package's own cgo LDFLAGS
# stay at -lvmmapi.
DEVLDFLAGS=	-L$$(pwd)/$(BUILDDIR) -lutil

all: build

# Build everything. Expects libvmmapi to be installed, or BUILDDIR to
# hold a locally built copy (see the libvmmapi target).
build:
	CGO_LDFLAGS="$(DEVLDFLAGS)" $(GO) build ./...

test:
	CGO_LDFLAGS="$(DEVLDFLAGS)" $(GO) test ./...

# The race detector is useful on the pure-Go layer. Integration tests
# that drive the kernel are tagged and excluded by default.
race:
	CGO_LDFLAGS="$(DEVLDFLAGS)" $(GO) test -race ./...

bench:
	CGO_LDFLAGS="$(DEVLDFLAGS)" $(GO) test -run '^$$' -bench . -benchmem ./...

vet:
	CGO_LDFLAGS="$(DEVLDFLAGS)" $(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint: vet cwarn
	@command -v staticcheck >/dev/null 2>&1 && \
	    CGO_LDFLAGS="$(DEVLDFLAGS)" staticcheck ./... || \
	    echo "staticcheck not installed, skipping"

# Compile the C bridge on its own with warnings as errors.
cwarn:
	$(CC) -c -o /dev/null $(CWARNFLAGS) -I internal/vmmapi \
	    internal/vmmapi/compat.c
	@echo "compat.c: clean under $(CWARNFLAGS)"

# Some FreeBSD installations are built without bhyve and so ship
# <vmmapi.h> but no libvmmapi to link against. This builds one from the
# system sources into build/ for development; nothing is vendored.
libvmmapi:
	@test -f $(LIBSRC)/vmmapi.c || { \
	    echo "$(LIBSRC)/vmmapi.c not found; install the system sources" \
	        "or set SRCTOP"; \
	    exit 1; \
	}
	mkdir -p $(BUILDDIR)
	$(CC) -O2 -fPIC -I$(LIBSRC) -c $(LIBSRC)/vmmapi.c \
	    -o $(BUILDDIR)/vmmapi.o
	$(CC) -O2 -fPIC -I$(LIBSRC) -c $(LIBSRC)/vmmapi_freebsd.c \
	    -o $(BUILDDIR)/vmmapi_freebsd.o
	ar rcs $(BUILDDIR)/libvmmapi.a $(BUILDDIR)/vmmapi.o \
	    $(BUILDDIR)/vmmapi_freebsd.o
	@echo "built $(BUILDDIR)/libvmmapi.a"

clean:
	rm -rf $(BUILDDIR)
	$(GO) clean ./...

.PHONY: all build test race bench vet fmt lint cwarn libvmmapi privileged clean

# Level 3 tests: they enter the guest and touch host hardware, so they
# need root and vmm(4) and are excluded from an ordinary run.
privileged:
	CGO_LDFLAGS="$(DEVLDFLAGS)" $(GO) test -tags vmm_privileged -v ./tests/privileged/
