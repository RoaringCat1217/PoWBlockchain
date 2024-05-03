# Set the Go compiler
GO := go

# Set the package name
PKGNAME := src

# Set the package directories
PKG_DIRS := $(PKGNAME)/blockchain $(PKGNAME)/miner $(PKGNAME)/tracker $(PKGNAME)/user $(PKGNAME)/tests

# Set the build flags
BUILD_FLAGS := -v

# Set the test flags
TEST_FLAGS := -v

# Set the test timeout
TEST_TIMEOUT := 120s

.PHONY: build test test-files test-packages clean docs

.SILENT: build test test-files test-packages clean docs

# Build the package
build:
	cd $(PKGNAME) $(GO) build

# Run all tests
test: test-files test-packages

# Run tests for individual files
test-files:
	@for pkg in $(PKG_DIRS); do \
		for file in $$(find $$pkg -name '*_test.go'); do \
			echo "Testing $$file"; \
			cd $$(dirname $$file); $(GO) test $(BUILD_FLAGS) -timeout $(TEST_TIMEOUT) ./; \
		done; \
	done

# Clean build artifacts and test cache
clean:
	rm -rf $(PKGNAME)/*.test
	$(GO) clean -testcache
