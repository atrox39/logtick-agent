async function fetchMetrics() {
  try {
      const response = await fetch('/api/current_metrics'); // Endpoint en Go
      const data = await response.json();

      // Actualizar el contenido de los spans directamente
      document.getElementById('display-agent-id').textContent = data.agent_id;
      document.getElementById('display-agent-name').textContent = data.agent_name;
      document.getElementById('display-cpu-percent').textContent = `${data.cpu_percent.toFixed(2)}%`; // Formatear CPU a 2 decimales
      document.getElementById('display-memory-used').textContent = `${data.memory_used_mb} MB`;
      document.getElementById('display-memory-free').textContent = `${data.memory_free_mb} MB`;

      // Formatear el timestamp a un formato legible
      const date = new Date(data.timestamp * 1000); // Multiplicar por 1000 porque el timestamp de Go es en segundos
      document.getElementById('display-timestamp').textContent = date.toLocaleString();

      // Actualizar el título de la página y el encabezado del agente
      document.title = `Agent - ${data.agent_name}`;
      document.getElementById('agent-name').textContent = `Estado del Agente: ${data.agent_name}`;

      // Quitar el mensaje de error si existía y establecer color de fondo (si es necesario)
      document.body.style.backgroundColor = '#f4f4f4';

  } catch (err) {
      console.error("Error al cargar métricas:", err); // Log el error para depuración
      document.getElementById('metrics-data').innerHTML = '<p style="color: red;">Error al cargar métricas. Intentando de nuevo...</p>';
      document.title = `Agent - Error`;
      document.getElementById('agent-name').textContent = `Estado del Agente: ¡ERROR!`;
  }
}

// Cargar métricas al inicio
fetchMetrics();

// Actualizar métricas cada 600ms
setInterval(fetchMetrics, 600);
