class OverlayMonitor {
    constructor() {
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = Infinity; // Never give up reconnecting
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 30000; // Cap at 30 seconds
        this.eventDisplayTimeout = null;
        this.eventDisplayDuration = 10000; // 10 seconds default
        this.isDisplayingEvent = false; // Track if an event is currently being displayed
        this.eventQueue = []; // Queue for incoming events
        this.heartbeatInterval = null;
        this.heartbeatTimeout = null;
        this.heartbeatIntervalTime = 30000; // Send heartbeat every 30 seconds
        this.heartbeatTimeoutTime = 20000; // Wait 20 seconds for heartbeat response
        this.isReconnecting = false;
        this.connectionStartTime = null;
        this.lastHeartbeatTime = null;
        
        this.initializeElements();
        this.connect();
        this.startConnectionMonitoring();
    }

    initializeElements() {
        this.statusIndicator = document.getElementById('statusIndicator');
        this.statusText = document.getElementById('statusText');
        this.overlayContainer = document.getElementById('overlayContainer');
        this.eventDisplay = document.getElementById('eventDisplay');
        this.connectionStatus = document.getElementById('connectionStatus');
    }

    connect() {
        try {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws`;
            
            this.updateStatus('connecting', 'Connecting...');
            this.ws = new WebSocket(wsUrl);
            
            this.ws.onopen = () => this.onOpen();
            this.ws.onmessage = (event) => this.onMessage(event);
            this.ws.onclose = (event) => this.onClose(event);
            this.ws.onerror = (error) => this.onError(error);
            
            this.isReconnecting = false;
        } catch (error) {
            console.error('WebSocket connection error:', error);
            this.updateStatus('disconnected', 'Connection failed');
            this.scheduleReconnect();
        }
    }

    onOpen() {
        console.log('WebSocket connected');
        this.updateStatus('connected', 'Connected');
        this.reconnectAttempts = 0;
        this.isReconnecting = false;
        this.connectionStartTime = Date.now();
        this.startHeartbeat();
    }

    onMessage(event) {
        try {
            const data = JSON.parse(event.data);
            this.handleMessage(data);
        } catch (error) {
            console.error('Error parsing message:', error);
        }
    }

    onClose(event) {
        console.log('WebSocket closed:', event.code, event.reason);
        this.updateStatus('disconnected', 'Disconnected');
        this.stopHeartbeat();
        
        // Always try to reconnect, regardless of close code
        this.scheduleReconnect();
    }

    onError(error) {
        console.error('WebSocket error:', error);
        this.updateStatus('disconnected', 'Connection error');
    }

    handleMessage(data) {
        console.log('Received message:', data);
        
        // Handle heartbeat responses
        if (data.type === 'pong') {
            this.lastHeartbeatTime = Date.now();
            if (this.heartbeatTimeout) {
                clearTimeout(this.heartbeatTimeout);
                this.heartbeatTimeout = null;
            }
            console.log('Heartbeat response received');
            return;
        }
        
        // Show both SQS messages and image_ready events, ignore connection messages for overlay
        if (data.type === 'sqs_message' || data.type === 'image_ready') {
            if (this.isDisplayingEvent) {
                // If an event is currently being displayed, queue this one
                console.log('Event is currently being displayed, queueing new event');
                this.eventQueue.push(data);
            } else {
                // Show the event immediately
                this.showEvent(data);
            }
        }
    }

    showEvent(eventData) {
        // Mark that we're displaying an event
        this.isDisplayingEvent = true;

        console.log('Displaying event for full 5 seconds:', eventData);

        // Create event content
        this.createEventContent(eventData);
        
        // Show the overlay with animation
        this.overlayContainer.classList.add('show');
        this.eventDisplay.classList.remove('slide-out');
        this.eventDisplay.classList.add('slide-in');
        
        // Auto-hide after specified duration and send acknowledgment
        this.eventDisplayTimeout = setTimeout(() => {
            this.hideEvent();
            this.sendEventAcknowledgment();
            this.processNextEvent(); // Process next event in queue if any
        }, this.eventDisplayDuration);
    }

    hideEvent() {
        this.overlayContainer.classList.remove('show');1
    }

    createEventContent(eventData) {
        const timestamp = new Date(eventData.timestamp).toLocaleString();
        
        if (eventData.type === 'image_ready' && eventData.data) {
            // Handle image_ready events with image display
            const username = eventData.data.username || 'Unknown User';
            const imagePath = eventData.data.imagePath || '';
            const messageId = eventData.data.messageId || '';
            const imageUrl = `${imagePath}`;
            
            this.eventDisplay.innerHTML = `
                <div class="image-overlay">
                    <img src="${imageUrl}" alt="Generated Image" class="overlay-image" />
                    <div class="username-display">${this.escapeHtml(username)}</div>
                </div>
            `;
        } else {
            // Handle other event types (sqs_message, etc.)
            let content = '';
            let messageId = '';
            let eventType = 'event';
            
            if (eventData.type === 'sqs_message' && eventData.data) {
                content = eventData.data.body || 'No content';
                messageId = eventData.data.messageId || '';
                eventType = 'sqs-message';
            } else {
                content = typeof eventData.data === 'string' ? eventData.data : JSON.stringify(eventData.data, null, 2);
            }

            // Try to parse and format JSON content for better display
            try {
                const parsedContent = JSON.parse(content);
                content = JSON.stringify(parsedContent, null, 2);
            } catch (e) {
                // Content is not JSON, keep as is
            }
            
            this.eventDisplay.innerHTML = `
                <div class="event-header">
                    <span class="event-type ${eventType}">${eventData.type.replace('_', ' ')}</span>
                    <span class="event-timestamp">${timestamp}</span>
                </div>
                <div class="event-content">${this.escapeHtml(content)}</div>
                ${messageId ? `<div class="event-id">ID: ${messageId}</div>` : ''}
            `;
        }
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    updateStatus(status, text) {
        this.statusIndicator.className = `status-indicator ${status}`;
        this.statusText.textContent = text;
    }

    scheduleReconnect() {
        if (this.isReconnecting) return; // Prevent multiple reconnection attempts
        
        this.isReconnecting = true;
        this.reconnectAttempts++;
        
        // Calculate delay with exponential backoff, capped at maxReconnectDelay
        const delay = Math.min(
            this.reconnectDelay * Math.pow(2, this.reconnectAttempts + 1),
            this.maxReconnectDelay
        );
        
        this.updateStatus('connecting', `Reconnecting in ${Math.ceil(delay/1000)}s... (attempt ${this.reconnectAttempts})`);
        
        setTimeout(() => {
            this.connect();
        }, delay);

        this.startHeartbeat();
    }

    // Public methods for configuration
    setEventDisplayDuration(duration) {
        this.eventDisplayDuration = duration;
    }

    showConnectionStatus() {
        this.connectionStatus.style.display = 'block';
    }

    hideConnectionStatus() {
        this.connectionStatus.style.display = 'none';
    }

    // Process the next event in the queue
    processNextEvent() {
        // Mark that we're no longer displaying an event
        this.isDisplayingEvent = false;
        
        // Check if there are queued events
        if (this.eventQueue.length > 0) {
            console.log(`Processing next event from queue (${this.eventQueue.length} events remaining)`);
            const nextEvent = this.eventQueue.shift(); // Remove first event from queue
            this.showEvent(nextEvent);
        } else {
            console.log('No more events in queue');
        }
    }

    // Send acknowledgment to server that event has been displayed
    sendEventAcknowledgment() {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            const acknowledgment = {
                type: 'event_acknowledged',
                timestamp: new Date().toISOString()
            };
            
            this.ws.send(JSON.stringify(acknowledgment));
            console.log('Event acknowledgment sent to server');
        } else {
            console.warn('Cannot send acknowledgment - WebSocket not connected');
        }
    }

    // Manual control methods
    forceHideEvent() {
        if (this.isDisplayingEvent) {
            // Clear the timeout
            if (this.eventDisplayTimeout) {
                clearTimeout(this.eventDisplayTimeout);
            }
            
            this.hideEvent();
            this.sendEventAcknowledgment(); // Send acknowledgment when manually hiding
            this.processNextEvent(); // Process next event in queue if any
        }
    }

    // Method to manually reconnect
    reconnect() {
        this.reconnectAttempts = 0;
        if (this.ws) {
            this.ws.close();
        }
        this.connect();

        this.startHeartbeat();
    }

    // Heartbeat methods
    startHeartbeat() {
        this.stopHeartbeat(); // Clear any existing heartbeat
        
        this.heartbeatInterval = setInterval(() => {
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                const ping = {
                    type: 'ping',
                    timestamp: new Date().toISOString()
                };
                
                this.ws.send(JSON.stringify(ping));
                console.log('Heartbeat ping sent');
                
                // Set timeout for heartbeat response
                this.heartbeatTimeout = setTimeout(() => {
                    console.warn('Heartbeat timeout - connection may be dead');
                    this.reconnect();
                }, this.heartbeatTimeoutTime);
            }
        }, this.heartbeatIntervalTime);
    }

    stopHeartbeat() {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
            this.heartbeatInterval = null;
        }
        if (this.heartbeatTimeout) {
            clearTimeout(this.heartbeatTimeout);
            this.heartbeatTimeout = null;
        }
    }

    // Connection monitoring
    startConnectionMonitoring() {
        // Monitor connection health every 5 seconds
        setInterval(() => {
            if (this.ws) {
                const now = Date.now();
                const connectionAge = this.connectionStartTime ? now - this.connectionStartTime : 0;
                const timeSinceLastHeartbeat = this.lastHeartbeatTime ? now - this.lastHeartbeatTime : 0;
                
                // Update status with connection info
                if (this.ws.readyState === WebSocket.OPEN) {
                    const ageMinutes = Math.floor(connectionAge / 60000);
                    const ageSeconds = Math.floor((connectionAge % 60000) / 1000);
                    let statusText = `Connected (${ageMinutes}m ${ageSeconds}s)`;
                    
                    if (this.lastHeartbeatTime && timeSinceLastHeartbeat > this.heartbeatIntervalTime * 2) {
                        statusText += ' - No heartbeat';
                    }
                    
                    this.updateStatus('connected', statusText);
                } else {
                    this.updateStatus('disconnected', 'Disconnected');
                    this.scheduleReconnect();
                }
            } else {
                this.updateStatus('disconnected', 'No connection');
                this.scheduleReconnect();
            }
        }, 5000);
    }
}

// Initialize the overlay monitor when the page loads
document.addEventListener('DOMContentLoaded', () => {
    window.overlayMonitor = new OverlayMonitor();
    
    // Add keyboard shortcuts for debugging (optional)
    document.addEventListener('keydown', (e) => {
        // Press 'D' to toggle debug connection status
        if (e.key.toLowerCase() === 'd') {
            const status = document.getElementById('connectionStatus');
            status.style.display = status.style.display === 'none' ? 'block' : 'none';
        }
        
        // Press 'H' to manually hide current event
        if (e.key.toLowerCase() === 'h') {
            window.overlayMonitor.forceHideEvent();
        }
        
        // Press 'R' to reconnect
        if (e.key.toLowerCase() === 'r') {
            window.overlayMonitor.reconnect();
        }
    });
});

// Add some global utility functions for debugging and configuration
window.overlayConfig = {
    setDisplayDuration: (duration) => window.overlayMonitor?.setEventDisplayDuration(duration),
    showConnectionStatus: () => window.overlayMonitor?.showConnectionStatus(),
    hideConnectionStatus: () => window.overlayMonitor?.hideConnectionStatus(),
    hideEvent: () => window.overlayMonitor?.forceHideEvent(),
    reconnect: () => window.overlayMonitor?.reconnect(),
    getConnectionStatus: () => window.overlayMonitor?.ws?.readyState
};
