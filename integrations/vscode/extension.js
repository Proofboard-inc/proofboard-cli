const cp = require('child_process');
const vscode = require('vscode');

function cliPath() {
    return process.env.PROOFBOARD_CLI_PATH || 'proofboard';
}

function announceWorkspace(workspacePath, editorName) {
    const child = cp.spawn(cliPath(), ['detect', '--workspace', workspacePath, '--editor', editorName], {
        env: process.env,
        stdio: 'ignore',
        detached: true,
    });
    child.unref();
}

async function announceOpenWorkspaces() {
    const folders = vscode.workspace.workspaceFolders || [];
    for (const folder of folders) {
        announceWorkspace(folder.uri.fsPath, 'vscode');
    }
}

function activate(context) {
    const disposable = vscode.commands.registerCommand('proofboard.detectWorkspace', announceOpenWorkspaces);
    context.subscriptions.push(disposable);
    void announceOpenWorkspaces();
}

function deactivate() {}

module.exports = {
    activate,
    deactivate,
};
