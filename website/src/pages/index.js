import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <div className="hero-banner">
      <div className="container">
        <h1 className="hero-title">
          <span className="hero-title-gradient">{siteConfig.title}</span>
        </h1>
        <p className="hero-subtitle">{siteConfig.tagline}</p>
        <div className="hero-buttons">
          <Link
            className="btn-primary"
            to="/docs/DEPLOYMENT_GUIDE">
            🚀 Comenzar Guía SRE
          </Link>
          <Link
            className="btn-secondary"
            to="/docs/COMMANDS">
            📖 Referencia CLI
          </Link>
        </div>

        {/* Premium Terminal Mockup */}
        <div className="terminal-mockup">
          <div className="terminal-header">
            <span className="terminal-dot dot-red"></span>
            <span className="terminal-dot dot-yellow"></span>
            <span className="terminal-dot dot-green"></span>
            <span className="terminal-title">kgg-admin@workstation:~</span>
          </div>
          <div className="terminal-body">
            <div className="terminal-line">
              <span className="terminal-prompt">$</span> kgg init
            </div>
            <div className="terminal-line" style={{ color: '#94a3b8' }}>
              ⚙️ Inicializando configurador interativo...
            </div>
            <div className="terminal-line" style={{ color: '#10b981' }}>
              ✔ Perfil de red y credenciales de host detectadas.
            </div>
            <div className="terminal-line" style={{ color: '#10b981' }}>
              ✔ Archivo 'kuargogo.yaml' generado con 3 nodos activos.
            </div>
            <div className="terminal-line">
              <span className="terminal-prompt">$</span> kgg site --tags k3s
            </div>
            <div className="terminal-line" style={{ color: '#60a5fa' }}>
              [ANSIBLE] Ejecutando rol: k3s-prep ...
            </div>
            <div className="terminal-line" style={{ color: '#60a5fa' }}>
              [ANSIBLE] Inicializando K3s Master en hp-master (192.168.1.101) ...
            </div>
            <div className="terminal-line" style={{ color: '#34d399' }}>
              ✔ ¡Cluster K3s desplegado correctamente!
            </div>
            <div className="terminal-line">
              <span className="terminal-prompt">$</span> <span className="terminal-success">kgg doctor</span>
            </div>
            <div className="terminal-line" style={{ color: '#34d399' }}>
              ✔ Nodos: 3/3 Online | Storage: OK | AI: Active (Ollama)
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout
      title={`${siteConfig.title} - Homelab Command Center`}
      description="Centro de mando CLI/TUI para homelabs y clústeres K3s en Raspberry Pi y x86.">
      <HomepageHeader />
      <main>
        <section className="features-section">
          <div className="features-grid">

            <div className="feature-card">
              <span className="feature-icon">⚡</span>
              <h3 className="feature-title">Aprovisionamiento Zero-Touch</h3>
              <p className="feature-description">
                Automatiza el ciclo de configuración de red, seguridad UFW, accesos SSH y preparación de almacenamiento persistente en tus nodos limpios de Debian.
              </p>
            </div>

            <div className="feature-card">
              <span className="feature-icon">🧠</span>
              <h3 className="feature-title">Diagnósticos con IA Local</h3>
              <p className="feature-description">
                Utiliza tu propio motor Ollama en GPU locales para analizar logs rebeldes, diagnosticar fallos en pods de Kubernetes y generar parches SRE al vuelo.
              </p>
            </div>

            <div className="feature-card">
              <span className="feature-icon">🤖</span>
              <h3 className="feature-title">Telegram "The Voice"</h3>
              <p className="feature-description">
                Recibe alertas en tiempo real, verifica el estado del cluster, ejecuta reboots seguros y consulta logs de journald a través de un daemon de Telegram seguro.
              </p>
            </div>

            <div className="feature-card">
              <span className="feature-icon">🛡️</span>
              <h3 className="feature-title">Auto-Recuperación Inteligente</h3>
              <p className="feature-description">
                Monitoreo continuo de salud del clúster sobre MQTT. Levantamiento automático mediante Wake-on-LAN para nodos inactivos y recuperación de réplicas en Longhorn.
              </p>
            </div>

          </div>
        </section>
      </main>
    </Layout>
  );
}
