#!/usr/bin/env fish

# Main deployment logic
function deploy
    echo "Building executable..."
    pushd discovery-server
    ./build.fish
    if test $status -ne 0
        echo "❌ Build failed."
        return 1
    end

    echo "Copying executable to server..."
    scp discovery-server server:~
    if test $status -ne 0
        echo "❌ SCP failed."
        return 1
    end
    popd

    echo "Deploying on server..."
    # The following block of commands is executed on the remote server via ssh.
    # It uses bash syntax (like 'set -e'), which is correct for that context.
    ssh server '
        # set -e
        echo "--> Stopping discovery-server service..."
        sudo systemctl stop discovery-server

        echo "--> Replacing executable..."
        sudo mv -f ~/discovery-server /opt/discovery-server

        echo "--> Starting discovery-server service..."
        sudo systemctl start discovery-server

        echo "--> Waiting for service to settle..."
        sleep 3

        echo "--> Checking service status..."
        if systemctl is-failed --quiet discovery-server; then
            echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
            echo "!!! Service FAILED to start.       !!!"
            echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
            journalctl -u discovery-server -n 20 --no-pager
            exit 1
        else
            echo "Service started successfully."
            systemctl status discovery-server --no-pager
        end
    '
    if test $status -ne 0
        echo "❌ Deployment script failed on server."
        return 1
    end

    return 0
end

# Run the deployment
deploy
if test $status -eq 0
    echo ""
    echo "✅ Deployment successful."
else
    echo ""
    echo "❌ Deployment process finished with an error."
end
