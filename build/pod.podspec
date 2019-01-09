Pod::Spec.new do |spec|
  spec.name         = 'getsc'
  spec.version      = '{{.Version}}'
  spec.license      = { :type => 'GNU Lesser General Public License, Version 3.0' }
  spec.homepage     = 'https://github.com/ETSC3259/etsc'
  spec.authors      = { {{range .Contributors}}
		'{{.Name}}' => '{{.Email}}',{{end}}
	}
  spec.summary      = 'iOS etsc Client'
  spec.source       = { :git => 'https://github.com/ETSC3259/etsc.git', :commit => '{{.Commit}}' }

	spec.platform = :ios
  spec.ios.deployment_target  = '9.0'
	spec.ios.vendored_frameworks = 'Frameworks/getsc.framework'

	spec.prepare_command = <<-CMD
    curl https://getscstore.blob.core.windows.net/builds/{{.Archive}}.tar.gz | tar -xvz
    mkdir Frameworks
    mv {{.Archive}}/getsc.framework Frameworks
    rm -rf {{.Archive}}
  CMD
end
