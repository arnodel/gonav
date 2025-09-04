#!/bin/bash

echo "Starting Go Navigator development environment..."

# Build and start the Go server in the background
echo "Building Go server..."
go build -o gonav .

echo "Starting Go server on port 8080..."
./gonav &
SERVER_PID=$!

# Start the frontend development server
echo "Starting frontend development server..."
cd frontend
npm run dev &
FRONTEND_PID=$!
cd ..

echo "Servers started!"
echo "Frontend: http://localhost:1234 (Parcel dev server)"
echo "Backend: http://localhost:8080 (Go API server)"
echo ""
echo "Press Ctrl+C to stop both servers"

# Function to cleanup on exit
cleanup() {
    echo ""
    echo "Stopping servers..."
    
    # Send SIGTERM to Go server (it will handle graceful shutdown)
    if kill -0 $SERVER_PID 2>/dev/null; then
        echo "Stopping Go server (PID: $SERVER_PID)..."
        kill -TERM $SERVER_PID 2>/dev/null
        
        # Wait up to 10 seconds for graceful shutdown
        for i in {1..10}; do
            if ! kill -0 $SERVER_PID 2>/dev/null; then
                echo "Go server stopped gracefully"
                break
            fi
            sleep 1
            if [ $i -eq 10 ]; then
                echo "Force killing Go server..."
                kill -KILL $SERVER_PID 2>/dev/null
            fi
        done
    fi
    
    # Kill frontend server
    if kill -0 $FRONTEND_PID 2>/dev/null; then
        echo "Stopping frontend server (PID: $FRONTEND_PID)..."
        kill -TERM $FRONTEND_PID 2>/dev/null
        sleep 2
        kill -KILL $FRONTEND_PID 2>/dev/null
    fi
    
    echo "All servers stopped"
    exit 0
}

# Set trap to cleanup on Ctrl+C and other signals
trap cleanup SIGINT SIGTERM

# Wait for user to stop
wait