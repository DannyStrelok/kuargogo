package config

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  ClusterConfig
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: ClusterConfig{
				Nodes: []Node{
					{Name: "master", IP: "192.168.1.10", Role: "master"},
					{Name: "worker", IP: "192.168.1.11", Role: "worker"},
				},
				SSH: SSH{PrivateKeyPath: "~/.ssh/id_rsa"},
			},
			wantErr: false,
		},
		{
			name: "Missing Master",
			config: ClusterConfig{
				Nodes: []Node{
					{Name: "worker", IP: "192.168.1.11", Role: "worker"},
				},
				SSH: SSH{PrivateKeyPath: "~/.ssh/id_rsa"},
			},
			wantErr: true,
		},
		{
			name: "Duplicate Name",
			config: ClusterConfig{
				Nodes: []Node{
					{Name: "node1", IP: "192.168.1.10", Role: "master"},
					{Name: "node1", IP: "192.168.1.11", Role: "worker"},
				},
				SSH: SSH{PrivateKeyPath: "~/.ssh/id_rsa"},
			},
			wantErr: true,
		},
		{
			name: "Duplicate IP",
			config: ClusterConfig{
				Nodes: []Node{
					{Name: "node1", IP: "192.168.1.10", Role: "master"},
					{Name: "node2", IP: "192.168.1.10", Role: "worker"},
				},
				SSH: SSH{PrivateKeyPath: "~/.ssh/id_rsa"},
			},
			wantErr: true,
		},
		{
			name: "Invalid IP",
			config: ClusterConfig{
				Nodes: []Node{
					{Name: "node1", IP: "999.999.999.999", Role: "master"},
				},
				SSH: SSH{PrivateKeyPath: "~/.ssh/id_rsa"},
			},
			wantErr: true,
		},
		{
			name: "Invalid Role",
			config: ClusterConfig{
				Nodes: []Node{
					{Name: "node1", IP: "192.168.1.10", Role: "superman"},
				},
				SSH: SSH{PrivateKeyPath: "~/.ssh/id_rsa"},
			},
			wantErr: true,
		},
		{
			name: "Missing SSH Key",
			config: ClusterConfig{
				Nodes: []Node{
					{Name: "master", IP: "192.168.1.10", Role: "master"},
				},
				SSH: SSH{PrivateKeyPath: ""},
			},
			wantErr: true,
		},
		{
			name: "Valid HA with VIP",
			config: ClusterConfig{
				Nodes: []Node{
					{Name: "m1", IP: "192.168.1.1", Role: "master"},
					{Name: "m2", IP: "192.168.1.2", Role: "master"},
				},
				K3s: K3s{VIP: "192.168.1.100"},
				SSH: SSH{PrivateKeyPath: "~/.ssh/id_rsa"},
			},
			wantErr: false,
		},
		{
			name: "Invalid HA missing VIP",
			config: ClusterConfig{
				Nodes: []Node{
					{Name: "m1", IP: "192.168.1.1", Role: "master"},
					{Name: "m2", IP: "192.168.1.2", Role: "master"},
				},
				SSH: SSH{PrivateKeyPath: "~/.ssh/id_rsa"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
