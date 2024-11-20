// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openvpn

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("#ClientConfig", func() {

	Describe("#GenerateClientConfig", func() {
		Context("non HA running in seed config", func() {
			cfg := ClientValues{
				Endpoint:       "api.something.local.gardener.cloud",
				VPNClientIndex: -1,
				OpenVPNPort:    1143,
				IsShootClient:  false,
			}
			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto tcp-client", func() {
					Expect(content).To(ContainSubstring(`proto tcp-client`))
				})

				It("tls config", func() {
					Expect(content).To(ContainSubstring(`key /srv/secrets/vpn-client/tls.key
cert /srv/secrets/vpn-client/tls.crt
ca /srv/secrets/vpn-client/ca.crt`))
				})
			})
		})

		Context("non HA running in shoot config", func() {
			cfg := ClientValues{
				Endpoint:          "api.something.local.gardener.cloud",
				VPNClientIndex:    -1,
				OpenVPNPort:       1143,
				ReversedVPNHeader: "invalid-host",
				IsShootClient:     true,
				SeedPodNetwork:    "10.123.0.0/19",
			}

			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto tcp-client", func() {
					Expect(content).To(ContainSubstring(`proto tcp-client`))
				})
				It("tls config", func() {
					Expect(content).To(ContainSubstring(`
key /srv/secrets/vpn-client/tls.key
cert /srv/secrets/vpn-client/tls.crt
ca /srv/secrets/vpn-client/ca.crt
`))
				})
				It("has http proxy options", func() {
					Expect(content).To(ContainSubstring(`
http-proxy api.something.local.gardener.cloud 1143
http-proxy-option CUSTOM-HEADER Reversed-VPN invalid-host`))
				})
				It("adds route for seed pod network", func() {
					Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/sh -c '/sbin/ip route replace 10.123.0.0/19 dev $1' -- "
`))
				})
			})

		})

		Context("HA config", func() {
			cfg := ClientValues{
				Endpoint:          "api.something.local.gardener.cloud",
				VPNClientIndex:    0,
				OpenVPNPort:       1143,
				ReversedVPNHeader: "invalid-host",
				IsShootClient:     true,
				SeedPodNetwork:    "2001:db8:77::/96",
			}

			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto tcp-client", func() {
					Expect(content).To(ContainSubstring(`proto tcp-client`))
				})
				It("tls config", func() {
					Expect(content).To(ContainSubstring(`
key /srv/secrets/vpn-client-0/tls.key
cert /srv/secrets/vpn-client-0/tls.crt
ca /srv/secrets/vpn-client-0/ca.crt
`))
				})

				It("adds route for seed pod network", func() {
					Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/sh -c '/sbin/ip route replace 2001:db8:77::/96 dev $1' -- "
`))
				})
			})
		})
	})
})
