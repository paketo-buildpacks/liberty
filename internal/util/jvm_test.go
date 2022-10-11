/*
 * Copyright 2018-2022 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package util_test

import (
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"
	"testing"

	. "github.com/onsi/gomega"
)

func testJVM(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
	)

	when("checking JVM name", func() {
		it("detects the OpenJ9 JVM", func() {
			executor := &mocks.Executor{}
			executor.On("Execute", mock.Anything).Run(func(args mock.Arguments) {
				arg := args.Get(0).(effect.Execution)
				_, err := arg.Stderr.Write([]byte(`
						java.vendor = IBM Corporation
						java.vendor.url = https://www.ibm.com/semeru-runtimes
						java.vendor.url.bug = https://github.com/ibmruntimes/Semeru-Runtimes/issues
						java.vendor.version = 11.0.16.1
						java.version = 11.0.16.1
						java.version.date = 2022-08-12
						java.vm.name = Eclipse OpenJ9 VM
						java.vm.vendor = Eclipse OpenJ9
						java.vm.version = openj9-0.33.1`),
				)
				Expect(err).ToNot(HaveOccurred())
			}).Return(nil)
			Expect(util.DetectJVMName(executor)).To(Equal("OpenJ9"))
		})

		it("detects the Bellsoft Liberica JVM", func() {
			executor := &mocks.Executor{}
			executor.On("Execute", mock.Anything).Run(func(args mock.Arguments) {
				arg := args.Get(0).(effect.Execution)
				_, err := arg.Stderr.Write([]byte(`
						java.vendor = BellSoft
						java.vendor.url = https://bell-sw.com/
						java.vendor.url.bug = https://bell-sw.com/support
						java.version = 11.0.16.1
						java.version.date = 2022-08-12
						java.vm.compressedOopsMode = 32-bit
						java.vm.info = mixed mode
						java.vm.name = OpenJDK 64-Bit Server VM
						java.vm.vendor = BellSoft
						java.vm.version = 11.0.16.1+1-LTS`),
				)
				Expect(err).ToNot(HaveOccurred())
			}).Return(nil)
			Expect(util.DetectJVMName(executor)).To(Equal("OpenJDK"))
		})
	})
}
